// Package ingest implements an engine for receiving transactions that need
// to be packaged into a collection.
package ingest

import (
	"errors"
	"fmt"
	"sync"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/fifoqueue"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/lifecycle"
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
	lm                   *lifecycle.LifecycleManager
	wg                   *sync.WaitGroup // used to wait for processing loop to exit
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
		lm:                   lifecycle.NewLifecycleManager(),
		wg:                   new(sync.WaitGroup),
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

	conduit, err := net.Register(engine.PushTransactions, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine: %w", err)
	}

	e.conduit = conduit

	return e, nil
}

// Start starts the engine by starting the message processing loop.
func (e *Engine) Start(ctx irrecoverable.SignalerContext) {
	e.lm.OnStart(func() {
		e.wg.Add(1)
		go e.loop(ctx)
	})
}

// Ready returns a ready channel that is closed once the engine has fully started.
func (e *Engine) Ready() <-chan struct{} {
	return e.lm.Started()
}

// Done returns a done channel that is closed once the engine has fully stopped.
func (e *Engine) Done() <-chan struct{} {
	e.lm.OnStop()
	return e.lm.Stopped()
}

// Process processes the given event from the node with the given origin ID in
// a blocking manner. It returns the potential processing error when done.
func (e *Engine) Process(channel network.Channel, originID flow.Identifier, event interface{}) error {
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

// ProcessLocal processes an event originating on the local node.
func (e *Engine) ProcessLocal(event interface{}) error {
	// all errors are unexpected
	return e.messageHandler.Process(e.me.NodeID(), event)
}

// SubmitLocal submits an event originating on the local node.
func (e *Engine) SubmitLocal(event interface{}) {
	err := e.ProcessLocal(event)
	if err != nil {
		e.log.Fatal().Err(err).Msg("internal error processing event")
	}
}

// Submit submits the given event from the node with the given origin ID
// for processing in a non-blocking manner. It returns instantly and logs
// a potential processing error internally when done.
func (e *Engine) Submit(channel network.Channel, originID flow.Identifier, event interface{}) {
	err := e.Process(channel, originID, event)
	if err != nil {
		e.log.Fatal().Err(err).Msg("internal error processing event")
	}
}

// loop is the main message processing loop for transaction messages.
func (e *Engine) loop(ctx irrecoverable.SignalerContext) {
	defer e.wg.Done()

	for {
		select {
		case <-ctx.Done():
			e.lm.OnStop()
		case <-e.lm.ShutdownSignal():
			return
		case <-e.messageHandler.GetNotifier():
			err := e.processAvailableMessages()
			if err != nil {
				// if an error reaches this point, it is unexpected
				ctx.Throw(err)
			}
		}
	}
}

// processAvailableMessages is called when the message queue is non-empty. It
// will process transactions while the queue is non-empty, then return.
func (e *Engine) processAvailableMessages() error {
	for {
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
		return engine.NewUnverifiableInputError("could not get reference block for transaction (%x): %w", txID, err)
	}

	// using the transaction's reference block, determine which cluster we're in.
	// if we don't know the reference block, we will fail when attempting to query the epoch.
	refEpoch := refSnapshot.Epochs().Current()

	epochCounter, err := refEpoch.Counter()
	if err != nil {
		return fmt.Errorf("could not get counter for reference epoch: %w", err)
	}
	clusters, err := refEpoch.Clustering()
	if err != nil {
		return fmt.Errorf("could not get clusters for reference epoch: %w", err)
	}

	// use the transaction pool for the epoch the reference block is part of
	pool := e.pools.ForEpoch(epochCounter)

	// short-circuit if we have already stored the transaction
	if pool.Has(txID) {
		e.log.Debug().Msg("received dupe transaction")
		return nil
	}

	// check if the transaction is valid
	err = e.transactionValidator.Validate(tx)
	if err != nil {
		return engine.NewInvalidInputErrorf("invalid transaction (%x): %w", txID, err)
	}

	// get the locally assigned cluster and the cluster responsible for the transaction
	txCluster, ok := clusters.ByTxID(txID)
	if !ok {
		return fmt.Errorf("could not get cluster responsible for tx: %x", txID)
	}

	// get the cluster we are in for the reference epoch
	localCluster, _, ok := clusters.ByNodeID(e.me.NodeID())
	if !ok {
		// if we aren't assigned to a cluster, check that we are a member of
		// the reference epoch
		refIdentities, err := refEpoch.InitialIdentities()
		if err != nil {
			return fmt.Errorf("could not get initial identities for reference epoch: %w", err)
		}

		if _, ok := refIdentities.ByNodeID(e.me.NodeID()); ok {
			// CAUTION: we are a member of the epoch, but have no assigned cluster!
			// This is an unexpected condition and indicates a protocol state invariant has been broken
			return fmt.Errorf("this node should have an assigned cluster in epoch (counter=%d), but has none", epochCounter)
		}
		return engine.NewUnverifiableInputError("this node is not assigned a cluster in epoch (counter=%d)", epochCounter)
	}

	localClusterFingerPrint := localCluster.Fingerprint()
	txClusterFingerPrint := txCluster.Fingerprint()
	log = log.With().
		Hex("local_cluster", logging.ID(localClusterFingerPrint)).
		Hex("tx_cluster", logging.ID(txClusterFingerPrint)).
		Logger()

	// if our cluster is responsible for the transaction, add it to our local mempool
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
