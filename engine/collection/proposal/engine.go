// Package proposal implements an engine for proposing and guaranteeing
// collections and submitting them to consensus nodes.
package proposal

import (
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/module/trace"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/protocol"
	"github.com/dapperlabs/flow-go/storage"
)

type Config struct {
	// ProposalPeriod the interval at which the engine will propose a collection.
	ProposalPeriod time.Duration
}

// Engine is the collection proposal engine, which packages pending
// transactions into collections and sends them to consensus nodes.
type Engine struct {
	unit        *engine.Unit
	conf        Config
	log         zerolog.Logger
	tracer      trace.Tracer
	con         network.Conduit
	me          module.Local
	state       protocol.State
	provider    network.Engine // provider engine to propagate guarantees
	pool        mempool.Transactions
	collections storage.Collections
	guarantees  storage.Guarantees
}

func New(
	log zerolog.Logger,
	conf Config,
	net module.Network,
	me module.Local,
	state protocol.State,
	tracer trace.Tracer,
	provider network.Engine,
	pool mempool.Transactions,
	collections storage.Collections,
	guarantees storage.Guarantees,
) (*Engine, error) {

	e := &Engine{
		unit:        engine.NewUnit(),
		conf:        conf,
		log:         log.With().Str("engine", "proposal").Logger(),
		me:          me,
		state:       state,
		tracer:      tracer,
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

// SendVote will send a vote to the desired node.
func (e *Engine) SendVote(blockID flow.Identifier, view uint64, sig crypto.Signature, recipientID flow.Identifier) error {

	// build the vote message
	vote := &messages.ClusterBlockVote{
		BlockID:   blockID,
		View:      view,
		Signature: sig,
	}

	err := e.con.Submit(vote, recipientID)
	if err != nil {
		return fmt.Errorf("could not send vote: %w", err)
	}

	return nil
}

// BroadcastProposal submits a cluster block proposal (effectively a proposal
// for the next collection) to all the collection nodes in our cluster.
func (e *Engine) BroadcastProposal(header *flow.Header) error {

	// first, check that we are the proposer of the block
	if header.ProposerID != e.me.NodeID() {
		return fmt.Errorf("cannot broadcast proposal with non-local proposer (%x)", header.ProposerID)
	}

	// retrieve the payload for the block
	// NOTE: relies on the fact that cluster payload hash is the ID of its collection
	collectionID := header.PayloadHash
	collection, err := e.collections.LightByID(collectionID)
	if err != nil {
		return fmt.Errorf("could not get payload for block: %w", err)
	}
	payload := cluster.Payload{Collection: *collection}

	// retrieve all consensus in our cluster
	// TODO filter by cluster
	recipients, err := e.state.AtBlockID(header.ParentID).Identities(filter.HasRole(flow.RoleCollection))
	if err != nil {
		return fmt.Errorf("could not get cluster members: %w", err)
	}

	// create the proposal message for the collection
	msg := &messages.ClusterBlockProposal{
		Header:  header,
		Payload: &payload,
	}

	err = e.con.Submit(msg, recipients.NodeIDs()...)
	if err != nil {
		return fmt.Errorf("could not broadcast proposal: %w", err)
	}

	return nil
}

func (e *Engine) propose() {

	for {
		select {
		case <-time.After(e.conf.ProposalPeriod):

			err := e.createProposal()
			if errors.Is(err, ErrEmptyTxpool) {
				e.log.Debug().Msg("skipping collection proposal due to empty txpool")
			} else if err != nil {
				e.log.Error().Err(err).Msg("failed to create new proposal")
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
