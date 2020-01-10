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
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/protocol"
	"github.com/dapperlabs/flow-go/utils/logging"
)

// Engine is the transaction ingestion engine, which ensures that new
// transactions are delegated to the correct collection cluster, and prepared
// to be included in a collection.
type Engine struct {
	unit      *engine.Unit
	log       zerolog.Logger
	con       network.Conduit
	me        module.Local
	state     protocol.State
	clusters  flow.ClusterList
	clusterID flow.ClusterID // my cluster ID
	pool      mempool.Transactions
}

// New creates a new collection ingest engine.
func New(log zerolog.Logger, net module.Network, state protocol.State, me module.Local, pool mempool.Transactions) (*Engine, error) {
	identities, err := state.Final().Identities(identity.HasRole(flow.RoleCollection))
	if err != nil {
		return nil, fmt.Errorf("could not get identities: %w", err)
	}

	clusters := protocol.Cluster(identities)
	clusterID := clusters.ClusterIDFor(me.NodeID())

	logger := log.With().
		Str("engine", "ingest").
		Str("my_cluster", clusterID.String()).
		Logger()

	e := &Engine{
		unit:      engine.NewUnit(),
		log:       logger,
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

// pocess processes engine events.
//
// Transactions are validated and routed to the correct cluster, then added
// to the transaction mempool.
func (e *Engine) process(originID flow.Identifier, event interface{}) error {
	switch ev := event.(type) {
	case *flow.Transaction:
		return e.onTransaction(originID, ev)
	default:
		return fmt.Errorf("invalid event type (%T)", event)
	}
}

// onTransaction handles receipt of a new transaction. This can be submitted
// from outside the system or routed from another collection node.
func (e *Engine) onTransaction(originID flow.Identifier, tx *flow.Transaction) error {

	log := e.log.With().
		Hex("origin_id", originID[:]).
		Hex("tx_id", logging.ID(tx)).
		Logger()

	log.Debug().Msg("transaction message received")

	err := e.validateTransaction(tx)
	if err != nil {
		return fmt.Errorf("invalid transaction: %w", err)
	}

	clusterID := protocol.Route(len(e.clusters), tx.ID())

	// tx is routed to my cluster, add to mempool
	if clusterID == e.clusterID {
		log.Debug().Msg("adding transaction to pool")
		return e.pool.Add(tx)
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
