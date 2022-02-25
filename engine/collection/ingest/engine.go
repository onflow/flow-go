// Package ingest implements an engine for receiving transactions that need
// to be packaged into a collection.
package ingest

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/fifoqueue"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
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
	*component.ComponentManager
	log                  zerolog.Logger
	engMetrics           module.EngineMetrics
	colMetrics           module.CollectionMetrics
	conduit              network.Conduit
	me                   module.Local
	state                protocol.State
	pendingTransactions  engine.MessageStore
	messageHandler       *engine.MessageHandler
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
	mempoolMetrics module.MempoolMetrics,
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

	// FIFO queue for transactions
	queue, err := fifoqueue.NewFifoQueue(
		fifoqueue.WithCapacity(int(config.MaxMessageQueueSize)),
		fifoqueue.WithLengthObserver(func(len int) {
			mempoolMetrics.MempoolEntries(metrics.ResourceTransactionIngestQueue, uint(len))
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("could not create transaction message queue: %w", err)
	}
	pendingTransactions := &engine.FifoMessageStore{FifoQueue: queue}

	// define how inbound messages are mapped to message queues
	handler := engine.NewMessageHandler(
		logger,
		engine.NewNotifier(),
		engine.Pattern{
			Match: func(msg *engine.Message) bool {
				_, ok := msg.Payload.(*flow.TransactionBody)
				if ok {
					engMetrics.MessageReceived(metrics.EngineCollectionIngest, metrics.MessageTransaction)
				}
				return ok
			},
			Store: pendingTransactions,
		},
	)

	e := &Engine{
		log:                  logger,
		engMetrics:           engMetrics,
		colMetrics:           colMetrics,
		me:                   me,
		state:                state,
		pendingTransactions:  pendingTransactions,
		messageHandler:       handler,
		pools:                pools,
		config:               config,
		transactionValidator: transactionValidator,
	}

	e.ComponentManager = component.NewComponentManagerBuilder().
		AddWorker(e.processQueuedTransactions).
		Build()

	conduit, err := net.Register(engine.PushTransactions, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine: %w", err)
	}
	e.conduit = conduit

	return e, nil
}

// Process processes a transaction message from the network and enqueues the
// message. Validation and ingestion is performed in the processQueuedTransactions
// worker.
func (e *Engine) Process(channel network.Channel, originID flow.Identifier, event interface{}) error {
	select {
	case <-e.ComponentManager.ShutdownSignal():
		return component.ErrComponentShutdown
	default:
	}

	err := e.messageHandler.Process(originID, event)
	if err != nil {
		if engine.IsIncompatibleInputTypeError(err) {
			e.log.Warn().Msgf("%v delivered unsupported message %T through %v", originID, event, channel)
			return nil
		}
		return fmt.Errorf("unexpected error while processing engine message: %w", err)
	}
	return nil
}

// ProcessTransaction processes a transaction message submitted from another
// local component. The transaction is validated and ingested synchronously.
// This is used by the GRPC API, for transactions from Access nodes.
func (e *Engine) ProcessTransaction(tx *flow.TransactionBody) error {
	return e.onTransaction(e.me.NodeID(), tx)
}

// processQueuedTransactions is the main message processing loop for transaction messages.
func (e *Engine) processQueuedTransactions(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	for {
		select {
		case <-ctx.Done():
			return
		case <-e.ComponentManager.ShutdownSignal():
			return
		case <-e.messageHandler.GetNotifier():
			err := e.processAvailableMessages(ctx)
			if err != nil {
				// if an error reaches this point, it is unexpected
				ctx.Throw(err)
			}
		}
	}
}

// processAvailableMessages is called when the message queue is non-empty. It
// will process transactions while the queue is non-empty, then return.
//
// All expected error conditions are handled within this function. Unexpected
// errors which should cause the component to stop are passed up.
func (e *Engine) processAvailableMessages(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		msg, ok := e.pendingTransactions.Get()
		if ok {
			err := e.onTransaction(msg.OriginID, msg.Payload.(*flow.TransactionBody))
			// log warnings for expected error conditions
			if engine.IsUnverifiableInputError(err) {
				e.log.Warn().Err(err).Msg("unable to process unverifiable transaction")
			} else if engine.IsInvalidInputError(err) {
				e.log.Warn().Err(err).Msg("discarding invalid transaction")
			} else if err != nil {
				// bubble up unexpected error
				return fmt.Errorf("unexpected error handling transaction: %w", err)
			}
			continue
		}

		// when there is no more messages in the queue, back to the loop to wait
		// for the next incoming message to arrive.
		return nil
	}
}

// onTransaction handles receipt of a new transaction. This can be submitted
// from outside the system or routed from another collection node.
func (e *Engine) onTransaction(originID flow.Identifier, tx *flow.TransactionBody) error {

	defer e.engMetrics.MessageHandled(metrics.EngineCollectionIngest, metrics.MessageTransaction)

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
		log.Debug().Msg("trace - cannot validate transaction with unknown reference block")
		return engine.NewUnverifiableInputError("could not get reference block for transaction (%x): %w", txID, err)
	}

	// using the transaction's reference block, determine which cluster we're in.
	// if we don't know the reference block, we will fail when attempting to query the epoch.
	refEpoch := refSnapshot.Epochs().Current()

	epochCounter, err := refEpoch.Counter()
	if err != nil {
		log.Debug().Msg("trace - cannot retrieve epoch counter for transaction")
		return fmt.Errorf("could not get counter for reference epoch: %w", err)
	}
	clusters, err := refEpoch.Clustering()
	if err != nil {
		log.Debug().Msg("trace - cannot retrieve clusters for transaction")
		return fmt.Errorf("could not get clusters for reference epoch: %w", err)
	}

	log.Debug().Msg("trace - retrieving pool for transaction")

	// use the transaction pool for the epoch the reference block is part of
	pool := e.pools.ForEpoch(epochCounter)

	log.Debug().Msg("trace - retrieved pool for transaction")

	// short-circuit if we have already stored the transaction
	if pool.Has(txID) {
		e.log.Debug().Msg("received dupe transaction")
		return nil
	}

	log.Debug().Msg("trace - transaction is not already present in mempool")

	// check if the transaction is valid
	err = e.transactionValidator.Validate(tx)
	if err != nil {
		log.Debug().Err(err).Msg("trace - transaction failed validation")
		return engine.NewInvalidInputErrorf("invalid transaction (%x): %w", txID, err)
	}

	log.Debug().Msg("trace - transaction passed validation")

	// get the locally assigned cluster and the cluster responsible for the transaction
	txCluster, ok := clusters.ByTxID(txID)
	if !ok {
		log.Debug().Msg("trace - no cluster for transaction")
		return fmt.Errorf("could not get cluster responsible for tx: %x", txID)
	}

	log.Debug().Msg("trace - got transaction cluster")

	// get the cluster we are in for the reference epoch
	localCluster, _, ok := clusters.ByNodeID(e.me.NodeID())
	if !ok {
		e.log.Debug().Msg("trace - no cluster found for this node")

		// if we aren't assigned to a cluster, check that we are a member of
		// the reference epoch
		refIdentities, err := refEpoch.InitialIdentities()
		if err != nil {
			e.log.Debug().Err(err).Msg("trace - cannot get initial identities")
			return fmt.Errorf("could not get initial identities for reference epoch: %w", err)
		}

		if _, ok := refIdentities.ByNodeID(e.me.NodeID()); ok {
			e.log.Debug().Msg("trace - cannot get cluster for this node when it should have one")
			// CAUTION: we are a member of the epoch, but have no assigned cluster!
			// This is an unexpected condition and indicates a protocol state invariant has been broken
			return fmt.Errorf("this node should have an assigned cluster in epoch (counter=%d), but has none", epochCounter)
		}
		e.log.Debug().Msg("trace - node not assigned cluster (expected case)")
		return engine.NewUnverifiableInputError("this node is not assigned a cluster in epoch (counter=%d)", epochCounter)
	}

	log.Debug().Msg("trace - got local cluster")

	localClusterFingerPrint := localCluster.Fingerprint()
	txClusterFingerPrint := txCluster.Fingerprint()
	log = log.With().
		Hex("local_cluster", logging.ID(localClusterFingerPrint)).
		Hex("tx_cluster", logging.ID(txClusterFingerPrint)).
		Logger()

	// if our cluster is responsible for the transaction, add it to our local mempool
	if localClusterFingerPrint == txClusterFingerPrint {
		log.Debug().Msg("trace - adding transaction to pool")
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
			// if multicast to a target cluster with at least one node failed, log an error and exit
			e.log.Error().Err(err).Msg("could not route transaction to cluster")
			return nil
		}
		if err == nil {
			e.engMetrics.MessageSent(metrics.EngineCollectionIngest, metrics.MessageTransaction)
		}
	}

	log.Info().Msg("transaction processed")

	return nil
}
