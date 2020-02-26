// Package proposal implements an engine for proposing and guaranteeing
// collections and submitting them to consensus nodes.
package proposal

import (
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/consensus/coldstuff"
	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/module/trace"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/protocol"
	"github.com/dapperlabs/flow-go/storage"
)

// Engine is the collection proposal engine, which packages pending
// transactions into collections and sends them to consensus nodes.
type Engine struct {
	unit        *engine.Unit
	log         zerolog.Logger
	tracer      trace.Tracer
	con         network.Conduit
	me          module.Local
	state       protocol.State
	provider    network.Engine // provider engine to propagate guarantees
	pool        mempool.Transactions
	collections storage.Collections
	guarantees  storage.Guarantees
	build       module.Builder
	finalizer   module.Finalizer

	coldstuff module.ColdStuff
}

func New(
	log zerolog.Logger,
	net module.Network,
	me module.Local,
	state protocol.State,
	tracer trace.Tracer,
	provider network.Engine,
	pool mempool.Transactions,
	collections storage.Collections,
	guarantees storage.Guarantees,
	build module.Builder,
	finalizer module.Finalizer,
) (*Engine, error) {

	e := &Engine{
		unit:        engine.NewUnit(),
		log:         log.With().Str("engine", "proposal").Logger(),
		me:          me,
		state:       state,
		tracer:      tracer,
		provider:    provider,
		pool:        pool,
		collections: collections,
		guarantees:  guarantees,
		build:       build,
		finalizer:   finalizer,
	}

	cold, err := coldstuff.New(log, state, me, e, build, finalizer, 3*time.Second, 8*time.Second)
	if err != nil {
		return nil, fmt.Errorf("could not initialize coldstuff: %w", err)
	}

	e.coldstuff = cold

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
	return e.unit.Ready(func() {
		<-e.coldstuff.Ready()
	})
}

// Done returns a done channel that is closed once the engine has fully stopped.
func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done(func() {
		<-e.coldstuff.Done()
	})
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

// createProposal creates a new proposal
func (e *Engine) createProposal() error {
	if e.pool.Size() == 0 {
		return ErrEmptyTxpool
	}

	transactions := e.pool.All()
	coll := flow.CollectionFromTransactions(transactions)

	err := e.collections.Store(&coll)
	if err != nil {
		return fmt.Errorf("could not save proposed collection: %w", err)
	}

	guarantee := coll.Guarantee()

	trace.StartCollectionGuaranteeSpan(e.tracer, guarantee, transactions).
		SetTag("node_type", "collection").
		SetTag("node_id", e.me.NodeID().String())

	err = e.guarantees.Store(&guarantee)
	if err != nil {
		return fmt.Errorf("could not save proposed collection guarantee %s: %w", guarantee.ID(), err)
	}

	// Collection guarantee is saved, we can now delete Txs from the mem pool
	for _, tx := range transactions {
		e.pool.Rem(tx.ID())
		e.tracer.FinishSpan(tx.ID())
	}

	err = e.provider.ProcessLocal(&messages.SubmitCollectionGuarantee{Guarantee: guarantee})
	if err != nil {
		return fmt.Errorf("could not submit collection guarantee: %w", err)
	}

	return nil
}
