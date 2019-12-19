// Package ingest implements an engine for receiving transactions that need
// to be packaged into a collection.
package ingest

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/identity"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/txpool"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/protocol"
)

// Engine is the transaction ingestion engine, which ensures that new
// transactions are delegated to the correct collection cluster, and prepared
// to be included in a collection.
type Engine struct {
	log       zerolog.Logger
	con       network.Conduit
	me        module.Local
	state     protocol.State
	clusters  flow.ClusterList
	clusterID flow.ClusterID // my cluster ID
	pool      *txpool.Pool   // TODO replace with merkle tree
}

// New creates a new collection ingest engine.
func New(log zerolog.Logger, net module.Network, me module.Local, state protocol.State, pool *txpool.Pool) (*Engine, error) {
	identities, err := state.Final().Identities(identity.HasRole(flow.RoleCollection))
	if err != nil {
		return nil, fmt.Errorf("could not get identities: %w", err)
	}

	clusters := protocol.Cluster(identities)
	clusterID := clusters.ClusterIDFor(me.NodeID())

	e := &Engine{
		log:       log.With().Str("engine", "ingest").Logger(),
		me:        me,
		state:     state,
		clusters:  clusters,
		clusterID: clusterID,
		pool:      pool,
	}

	con, err := net.Register(engine.CollectionIngest, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine: %w", err)
	}

	e.con = con

	return e, nil
}

// Ready returns a ready channel that is closed once the engine has fully
// started.
// TODO describe condition for ingest engine being ready
func (e *Engine) Ready() <-chan struct{} {
	// TODO implement
	ready := make(chan struct{})
	go func() {
		close(ready)
	}()
	return ready
}

// Done returns a done channel that is closed once the engine has fully stopped.
// TODO describe conditions under which engine is done
func (e *Engine) Done() <-chan struct{} {
	// TODO implement
	done := make(chan struct{})
	go func() {
		close(done)
	}()
	return done
}

// Submit allows us to submit local events to the engine.
func (e *Engine) Submit(event interface{}) error {
	err := e.Process(e.me.NodeID(), event)
	return err
}

// Process processes engine events.
//
// Transactions are validated and routed to the correct cluster, then added
// to the transaction mempool.
func (e *Engine) Process(originID flow.Identifier, event interface{}) error {
	var err error
	switch ev := event.(type) {
	case *flow.Transaction:
		err = e.onTransaction(originID, ev)
	default:
		err = fmt.Errorf("invalid event type (%T)", event)
	}
	if err != nil {
		return fmt.Errorf("could not process event: %w", err)
	}
	return nil
}

// onTransaction handles receipt of a new transaction. This can be submitted
// from outside the system or routed from another collection node.
func (e *Engine) onTransaction(originID flow.Identifier, tx *flow.Transaction) error {
	log := e.log.With().
		Hex("origin_id", originID[:]).
		Hex("tx_hash", tx.Fingerprint()).
		Logger()

	log.Debug().Msg("transaction message received")

	err := e.validateTransaction(tx)
	if err != nil {
		return fmt.Errorf("invalid transaction: %w", err)
	}

	clusterID := protocol.Route(len(e.clusters), tx.Hash())

	// tx is routed to my cluster, add to mempool
	if clusterID == e.clusterID {
		log.Debug().Msg("adding transaction to pool")
		e.pool.Add(tx)
		return nil
	}

	// tx is routed to another cluster
	cluster := e.clusters.Get(clusterID)
	if len(cluster) == 0 {
		return fmt.Errorf("transaction routed to invalid cluster")
	}

	// TODO Don't need to send to all nodes
	err = e.con.Submit(tx, cluster.NodeIDs()...)
	if err != nil {
		return fmt.Errorf("failed to route transaction to cluster nodes")
	}

	log.Debug().
		Int("target_cluster", int(clusterID)).
		Msg("routed transaction to cluster")

	return nil
}

// TODO: implement
func (e *Engine) validateTransaction(tx *flow.Transaction) error {
	missingFields := tx.MissingFields()
	if len(missingFields) > 0 {
		return ErrIncompleteTransaction{missing: missingFields}
	}

	// TODO check account/payer signatures

	return nil
}
