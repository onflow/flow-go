// Package ingest implements an engine for receiving transactions that need
// to be packaged into a collection.
package ingest

import (
	"errors"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool/epochs"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/utils/logging"
)

// Engine is the transaction ingestion engine, which ensures that new
// transactions are delegated to the correct collection cluster, and prepared
// to be included in a collection.
type Engine struct {
	unit                 *engine.Unit
	log                  zerolog.Logger
	engMetrics           module.EngineMetrics
	colMetrics           module.CollectionMetrics
	conduit              network.Conduit
	me                   module.Local
	state                protocol.State
	pools                *epochs.TransactionPools
	transactionValidator *access.TransactionValidator

	config Config
}

// New creates a new collection ingest engine.
func New(
	log zerolog.Logger,
	net network.Network,
	state protocol.State,
	engMetrics module.EngineMetrics,
	colMetrics module.CollectionMetrics,
	me module.Local,
	chain flow.Chain,
	pools *epochs.TransactionPools,
	config Config,
) (*Engine, error) {

	logger := log.With().Str("engine", "ingest").Logger()

	transactionValidator := access.NewTransactionValidator(
		access.NewProtocolStateBlocks(state),
		chain,
		access.TransactionValidationOptions{
			Expiry:                 flow.DefaultTransactionExpiry,
			ExpiryBuffer:           config.ExpiryBuffer,
			MaxGasLimit:            config.MaxGasLimit,
			MaxAddressIndex:        config.MaxAddressIndex,
			CheckScriptsParse:      config.CheckScriptsParse,
			MaxTransactionByteSize: config.MaxTransactionByteSize,
			MaxCollectionByteSize:  config.MaxCollectionByteSize,
		},
	)

	e := &Engine{
		unit:                 engine.NewUnit(),
		log:                  logger,
		engMetrics:           engMetrics,
		colMetrics:           colMetrics,
		me:                   me,
		state:                state,
		pools:                pools,
		config:               config,
		transactionValidator: transactionValidator,
	}

	conduit, err := net.Register(engine.PushTransactions, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine: %w", err)
	}

	e.conduit = conduit

	return e, nil
}

// Ready returns a ready channel that is closed once the engine has fully
// started.
func (e *Engine) Ready() <-chan struct{} {
	return e.unit.Ready()
}

// Done returns a done channel that is closed once the engine has fully stopped.
func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done()
}

// SubmitLocal submits an event originating on the local node.
func (e *Engine) SubmitLocal(event interface{}) {
	e.unit.Launch(func() {
		err := e.process(e.me.NodeID(), event)
		if err != nil {
			engine.LogError(e.log, err)
		}
	})
}

// Submit submits the given event from the node with the given origin ID
// for processing in a non-blocking manner. It returns instantly and logs
// a potential processing error internally when done.
func (e *Engine) Submit(channel network.Channel, originID flow.Identifier, event interface{}) {
	e.unit.Launch(func() {
		err := e.process(originID, event)
		if err != nil {
			engine.LogError(e.log, err)
		}
	})
}

// ProcessLocal processes an event originating on the local node.
func (e *Engine) ProcessLocal(event interface{}) error {
	return e.unit.Do(func() error {
		return e.process(e.me.NodeID(), event)
	})
}

// Process processes the given event from the node with the given origin ID in
// a blocking manner. It returns the potential processing error when done.
func (e *Engine) Process(channel network.Channel, originID flow.Identifier, event interface{}) error {
	return e.unit.Do(func() error {
		return e.process(originID, event)
	})
}

// process processes engine events.
//
// Transactions are validated and routed to the correct cluster, then added
// to the transaction mempool.
func (e *Engine) process(originID flow.Identifier, event interface{}) error {
	switch ev := event.(type) {
	case *flow.TransactionBody:
		e.engMetrics.MessageReceived(metrics.EngineCollectionIngest, metrics.MessageTransaction)
		defer e.engMetrics.MessageHandled(metrics.EngineCollectionIngest, metrics.MessageTransaction)
		return e.onTransaction(originID, ev)
	default:
		return fmt.Errorf("invalid event type (%T)", event)
	}
}

// onTransaction handles receipt of a new transaction. This can be submitted
// from outside the system or routed from another collection node.
func (e *Engine) onTransaction(originID flow.Identifier, tx *flow.TransactionBody) error {

	txID := tx.ID()

	log := e.log.With().
		Hex("origin_id", originID[:]).
		Hex("tx_id", txID[:]).
		Hex("ref_block_id", tx.ReferenceBlockID[:]).
		Logger()

	log.Info().Msg("transaction message received")

	// get the state snapshot w.r.t. the reference block
	refSnapshot := e.state.AtBlockID(tx.ReferenceBlockID)
	// fail fast if this is an unknown reference
	_, err := refSnapshot.Head()
	if err != nil {
		return fmt.Errorf("could not get reference block: %w", err)
	}

	// using the transaction's reference block, determine which cluster we're in.
	// if we don't know the reference block, we will fail when attempting to query the epoch.
	refEpoch := refSnapshot.Epochs().Current()

	counter, err := refEpoch.Counter()
	if err != nil {
		return fmt.Errorf("could not get counter for reference epoch: %w", err)
	}
	clusters, err := refEpoch.Clustering()
	if err != nil {
		return fmt.Errorf("could not get clusters for reference epoch: %w", err)
	}

	// use the transaction pool for the epoch the reference block is part of
	pool := e.pools.ForEpoch(counter)

	// short-circuit if we have already stored the transaction
	if pool.Has(txID) {
		e.log.Debug().Msg("received dupe transaction")
		return nil
	}

	// check if the transaction is valid
	err = e.transactionValidator.Validate(tx)
	if err != nil {
		return engine.NewInvalidInputErrorf("invalid transaction: %w", err)
	}

	// get the locally assigned cluster and the cluster responsible for the transaction
	txCluster, ok := clusters.ByTxID(txID)
	if !ok {
		return fmt.Errorf("could not get cluster responsible for tx: %x", txID)
	}

	// if we are not yet a member of any cluster, for example if we are joining
	// the network in the next epoch, we will return an error here
	localCluster, _, ok := clusters.ByNodeID(e.me.NodeID())
	if !ok {
		return fmt.Errorf("node is not assigned to any cluster in this epoch: %d", counter)
	}

	localClusterFingerPrint := localCluster.Fingerprint()
	txClusterFingerPrint := txCluster.Fingerprint()

	log = log.With().
		Hex("local_cluster", logging.ID(localClusterFingerPrint)).
		Hex("tx_cluster", logging.ID(txClusterFingerPrint)).
		Logger()

	// if our cluster is responsible for the transaction, add it to the mempool
	if localClusterFingerPrint == txClusterFingerPrint {
		_ = pool.Add(tx)
		e.colMetrics.TransactionIngested(txID)
		log.Debug().Msg("added transaction to pool")
	}

	// if the message was submitted internally (ie. via the Access API)
	// propagate it to members of the responsible cluster (either our cluster
	// or a different cluster)
	if originID == e.me.NodeID() {

		log.Debug().Msg("propagating transaction to cluster")

		err := e.conduit.Multicast(tx, e.config.PropagationRedundancy+1, txCluster.NodeIDs()...)
		if err != nil && !errors.Is(err, network.EmptyTargetList) {
			// if multicast to a target cluster with at least one node failed, return an error
			return fmt.Errorf("could not route transaction to cluster: %w", err)
		}
		if err == nil {
			e.engMetrics.MessageSent(metrics.EngineCollectionIngest, metrics.MessageTransaction)
		}
	}

	log.Info().Msg("transaction processed")

	return nil
}
