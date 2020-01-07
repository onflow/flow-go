// Package proposal implements an engine for proposing and guaranteeing
// collections and submitting them to consensus nodes.
package proposal

import (
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/protocol"
	"github.com/dapperlabs/flow-go/storage"
)

type Config struct {
	// ProposalPeriod the interval at which the engine will propose a collection.
	ProposalPerid time.Duration
}

// Engine is the collection proposal engine, which packages pending
// transactions into collections and sends them to consensus nodes.
type Engine struct {
	unit        *engine.Unit
	conf        Config
	log         zerolog.Logger
	con         network.Conduit
	me          module.Local
	state       protocol.State
	provider    network.Engine // provider engine to propagate guarantees
	pool        module.TransactionPool
	collections storage.Collections
	guarantees  storage.Guarantees
}

func New(
	log zerolog.Logger,
	conf Config,
	net module.Network,
	me module.Local,
	state protocol.State,
	provider network.Engine,
	pool module.TransactionPool,
	collections storage.Collections,
	guarantees storage.Guarantees,
) (*Engine, error) {

	e := &Engine{
		unit:        engine.NewUnit(),
		conf:        conf,
		log:         log.With().Str("engine", "proposal").Logger(),
		me:          me,
		state:       state,
		provider:    provider,
		pool:        pool,
		collections: collections,
		guarantees:  guarantees,
	}

	con, err := net.Register(engine.CollectionProposal, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine: %w", err)
	}

	e.con = con

	return e, nil
}

// Ready returns a ready channel that is closed once the engine has fully
// started.
func (e *Engine) Ready() <-chan struct{} {
	e.unit.Launch(e.propose)
	return e.unit.Ready()
}

// Done returns a done channel that is closed once the engine has fully stopped.
// TODO describe conditions under which engine is done
func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done()
}

// SubmitLocal submits an event originating on the local node.
func (e *Engine) SubmitLocal(event interface{}) {
	e.Submit(e.me.NodeID(), event)
}

// Submit submits the given event from the node with the given origin ID
// for processing in a non-blocking manner. It returns instantly and logs
// a potential processing error internally when done.
func (e *Engine) Submit(originID flow.Identifier, event interface{}) {
	e.unit.Launch(func() {
		err := e.Process(originID, event)
		if err != nil {
			e.log.Error().Err(err).Msg("could not process submitted event")
		}
	})
}

// ProcessLocal processes an event originating on the local node.
func (e *Engine) ProcessLocal(event interface{}) error {
	return e.Process(e.me.NodeID(), event)
}

// Process processes the given event from the node with the given origin ID in
// a blocking manner. It returns the potential processing error when done.
func (e *Engine) Process(originID flow.Identifier, event interface{}) error {
	return e.unit.Do(func() error {
		return e.process(originID, event)
	})
}

// process processes events for the proposal engine on the collection node.
func (e *Engine) process(originID flow.Identifier, event interface{}) error {
	switch event.(type) {
	default:
		return fmt.Errorf("invalid event type (%T)", event)
	}
}

func (e *Engine) propose() {

	ticker := time.NewTicker(e.conf.ProposalPerid)

	for {
		select {
		case <-ticker.C:

			err := e.createProposal()
			if err != nil {
				e.log.Err(err).Msg("failed to create new proposal")
				continue
			}

		case <-e.unit.Quit():
			return
		}
	}
}

// createProposal creates a new proposal
func (e *Engine) createProposal() error {
	if e.pool.Size() == 0 {
		return ErrEmptyTxpool
	}

	transactions := e.pool.All()
	coll := flow.CollectionFromTransactions(transactions)

	err := e.collections.Save(&coll)
	if err != nil {
		return fmt.Errorf("could not save proposed collection: %w", err)
	}

	guarantee := coll.Guarantee()
	err = e.guarantees.Save(&guarantee)
	if err != nil {
		return fmt.Errorf("could not save proposed collection guarantee: %w", err)
	}

	err = e.provider.ProcessLocal(&messages.SubmitCollectionGuarantee{Guarantee: guarantee})
	if err != nil {
		return fmt.Errorf("could not submit collection guarantee: %w", err)
	}

	return nil
}
