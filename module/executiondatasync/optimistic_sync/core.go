package optimistic_sync

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"

	"github.com/onflow/flow-go/engine/access/ingestion/tx_error_messages"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync/persisters"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync/persisters/stores"
	"github.com/onflow/flow-go/module/state_synchronization/indexer"
	"github.com/onflow/flow-go/module/state_synchronization/requester"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/inmemory"
)

// DefaultTxResultErrMsgsRequestTimeout is the default timeout for requesting transaction result error messages.
const DefaultTxResultErrMsgsRequestTimeout = 5 * time.Second

// errResultAbandoned is returned when calling one of the methods after the result has been abandoned.
// Not exported because this is not an expected error condition.
var errResultAbandoned = fmt.Errorf("result abandoned")

// <component_spec>
// Core defines the interface for pipelined execution result processing. There are 3 main steps which
// must be completed sequentially and exactly once.
// 1. Download the BlockExecutionData and TransactionResultErrorMessages for the execution result.
// 2. Index the downloaded data into mempools.
// 3. Persist the indexed data to into persisted storage.
//
// If the protocol abandons the execution result, Abandon() is called to signal to the Core instance
// that processing will stop and any data accumulated may be discarded. Abandon() may be called at
// any time, but may block until in-progress operations are complete.
// </component_spec>
//
// All exported methods are safe for concurrent use.
type Core interface {
	// Download retrieves all necessary data for processing from the network.
	// Download will block until the data is successfully downloaded, and has not internal timeout.
	// When Aboandon is called, the caller must cancel the context passed in to shutdown the operation
	// otherwise it may block indefinitely.
	//
	// Expected error returns during normal operation:
	// - [context.Canceled]: if the provided context was canceled before completion
	Download(ctx context.Context) error

	// Index processes the downloaded data and stores it into in-memory indexes.
	// Must be called after Download.
	//
	// No error returns are expected during normal operations
	Index() error

	// Persist stores the indexed data in permanent storage.
	// Must be called after Index.
	//
	// No error returns are expected during normal operations
	Persist() error

	// Abandon indicates that the protocol has abandoned this state. Hence processing will be aborted
	// and any data dropped.
	// This method will block until other in-progress operations are complete. If Download is in progress,
	// the caller should cancel its context to ensure the operation completes in a timely manner.
	Abandon()
}

// workingData encapsulates all components and temporary storage
// involved in processing a single block's execution data. When processing
// is complete or abandoned, the entire workingData can be discarded.
type workingData struct {
	protocolDB  storage.DB
	lockManager storage.LockManager

	persistentRegisters         storage.RegisterIndex
	persistentEvents            storage.Events
	persistentCollections       storage.Collections
	persistentResults           storage.LightTransactionResults
	persistentTxResultErrMsgs   storage.TransactionResultErrorMessages
	latestPersistedSealedResult storage.LatestPersistedSealedResult

	inmemRegisters       *inmemory.RegistersReader
	inmemEvents          *inmemory.EventsReader
	inmemCollections     *inmemory.CollectionsReader
	inmemTransactions    *inmemory.TransactionsReader
	inmemResults         *inmemory.LightTransactionResultsReader
	inmemTxResultErrMsgs *inmemory.TransactionResultErrorMessagesReader

	// Active processing components
	execDataRequester             requester.ExecutionDataRequester
	txResultErrMsgsRequester      tx_error_messages.Requester
	txResultErrMsgsRequestTimeout time.Duration

	// Working data
	executionData       *execution_data.BlockExecutionData
	txResultErrMsgsData []flow.TransactionResultErrorMessage
	indexerData         *indexer.IndexerData
	persisted           bool
}

var _ Core = (*CoreImpl)(nil)

// CoreImpl implements the Core interface for processing execution data.
// It coordinates the download, indexing, and persisting of execution data.
//
// Safe for concurrent use.
type CoreImpl struct {
	log zerolog.Logger
	mu  sync.Mutex

	workingData *workingData

	executionResult *flow.ExecutionResult
	block           *flow.Block
}

// NewCoreImpl creates a new CoreImpl with all necessary dependencies
// Safe for concurrent use.
//
// No error returns are expected during normal operations
func NewCoreImpl(
	logger zerolog.Logger,
	executionResult *flow.ExecutionResult,
	block *flow.Block,
	execDataRequester requester.ExecutionDataRequester,
	txResultErrMsgsRequester tx_error_messages.Requester,
	txResultErrMsgsRequestTimeout time.Duration,
	persistentRegisters storage.RegisterIndex,
	persistentEvents storage.Events,
	persistentCollections storage.Collections,
	persistentResults storage.LightTransactionResults,
	persistentTxResultErrMsg storage.TransactionResultErrorMessages,
	latestPersistedSealedResult storage.LatestPersistedSealedResult,
	protocolDB storage.DB,
	lockManager storage.LockManager,
) (*CoreImpl, error) {
	if block.ID() != executionResult.BlockID {
		return nil, fmt.Errorf("header ID and execution result block ID must match")
	}

	coreLogger := logger.With().
		Str("component", "execution_data_core").
		Str("execution_result_id", executionResult.ID().String()).
		Str("block_id", executionResult.BlockID.String()).
		Uint64("height", block.Height).
		Logger()

	return &CoreImpl{
		log:             coreLogger,
		block:           block,
		executionResult: executionResult,
		workingData: &workingData{
			protocolDB:  protocolDB,
			lockManager: lockManager,

			execDataRequester:             execDataRequester,
			txResultErrMsgsRequester:      txResultErrMsgsRequester,
			txResultErrMsgsRequestTimeout: txResultErrMsgsRequestTimeout,

			persistentRegisters:         persistentRegisters,
			persistentEvents:            persistentEvents,
			persistentCollections:       persistentCollections,
			persistentResults:           persistentResults,
			persistentTxResultErrMsgs:   persistentTxResultErrMsg,
			latestPersistedSealedResult: latestPersistedSealedResult,
		},
	}, nil
}

// Download retrieves all necessary data for processing from the network.
// Download will block until the data is successfully downloaded, and has not internal timeout.
// When Aboandon is called, the caller must cancel the context passed in to shutdown the operation
// otherwise it may block indefinitely.
//
// The method may only be called once. Calling it multiple times will return an error.
// Calling Download after Abandon is called will return an error.
//
// Expected error returns during normal operation:
// - [context.Canceled]: if the provided context was canceled before completion
func (c *CoreImpl) Download(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.workingData == nil {
		return errResultAbandoned
	}
	if c.workingData.executionData != nil {
		return fmt.Errorf("already downloaded")
	}

	c.log.Debug().Msg("downloading execution data")

	g, gCtx := errgroup.WithContext(ctx)

	var executionData *execution_data.BlockExecutionData
	g.Go(func() error {
		var err error
		executionData, err = c.workingData.execDataRequester.RequestExecutionData(gCtx)
		if err != nil {
			return fmt.Errorf("failed to request execution data: %w", err)
		}

		return nil
	})

	var txResultErrMsgsData []flow.TransactionResultErrorMessage
	g.Go(func() error {
		timeoutCtx, cancel := context.WithTimeout(gCtx, c.workingData.txResultErrMsgsRequestTimeout)
		defer cancel()

		var err error
		txResultErrMsgsData, err = c.workingData.txResultErrMsgsRequester.Request(timeoutCtx)
		if err != nil {
			// transaction error messages are downloaded from execution nodes over grpc and have no
			// protocol guarantees for delivery or correctness. Therefore, we attempt to download them
			// on a best-effort basis, and give up after a reasonable timeout to avoid blocking the
			// main indexing process. Missing error messages are handled gracefully by the rest of
			// the system, and can be retried or backfilled as needed later.
			if errors.Is(err, context.DeadlineExceeded) {
				c.log.Debug().
					Dur("timeout", c.workingData.txResultErrMsgsRequestTimeout).
					Msg("transaction result error messages request timed out")
				return nil
			}

			return fmt.Errorf("failed to request transaction result error messages data: %w", err)
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		return err
	}

	c.workingData.executionData = executionData
	c.workingData.txResultErrMsgsData = txResultErrMsgsData

	c.log.Debug().Msg("successfully downloaded execution data")

	return nil
}

// Index processes the downloaded data and stores it into in-memory indexes.
// Must be called after Download.
//
// The method may only be called once. Calling it multiple times will return an error.
// Calling Index after Abandon is called will return an error.
//
// No error returns are expected during normal operations
func (c *CoreImpl) Index() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.workingData == nil {
		return errResultAbandoned
	}
	if c.workingData.executionData == nil {
		return fmt.Errorf("downloading is not complete")
	}
	if c.workingData.indexerData != nil {
		return fmt.Errorf("already indexed")
	}

	c.log.Debug().Msg("indexing execution data")

	indexerComponent, err := indexer.NewInMemoryIndexer(c.log, c.block, c.executionResult)
	if err != nil {
		return fmt.Errorf("failed to create indexer: %w", err)
	}

	indexerData, err := indexerComponent.IndexBlockData(c.workingData.executionData)
	if err != nil {
		return fmt.Errorf("failed to index execution data: %w", err)
	}

	if c.workingData.txResultErrMsgsData != nil {
		err = indexer.ValidateTxErrors(indexerData.Results, c.workingData.txResultErrMsgsData)
		if err != nil {
			return fmt.Errorf("failed to validate transaction result error messages: %w", err)
		}
	}

	blockID := c.executionResult.BlockID

	c.workingData.indexerData = indexerData
	c.workingData.inmemCollections = inmemory.NewCollections(indexerData.Collections)
	c.workingData.inmemTransactions = inmemory.NewTransactions(indexerData.Transactions)
	c.workingData.inmemTxResultErrMsgs = inmemory.NewTransactionResultErrorMessages(blockID, c.workingData.txResultErrMsgsData)
	c.workingData.inmemEvents = inmemory.NewEvents(blockID, indexerData.Events)
	c.workingData.inmemResults = inmemory.NewLightTransactionResults(blockID, indexerData.Results)
	c.workingData.inmemRegisters = inmemory.NewRegisters(c.block.Height, indexerData.Registers)

	c.log.Debug().Msg("successfully indexed execution data")

	return nil
}

// Persist stores the indexed data in permanent storage.
// Must be called after Index.
//
// The method may only be called once. Calling it multiple times will return an error.
// Calling Persist after Abandon is called will return an error.
//
// No error returns are expected during normal operations
func (c *CoreImpl) Persist() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.workingData == nil {
		return errResultAbandoned
	}
	if c.workingData.persisted {
		return fmt.Errorf("already persisted")
	}
	if c.workingData.indexerData == nil {
		return fmt.Errorf("indexing is not complete")
	}

	c.log.Debug().Msg("persisting execution data")

	indexerData := c.workingData.indexerData

	// the BlockPersister updates the latest persisted sealed result within the batch operation, so
	// all other updates must be done before the batch is committed to ensure the state remains
	// consistent. The registers db allows repeated indexing of the most recent block's registers,
	// so it is safe to persist them before the block persister.
	registerPersister := persisters.NewRegistersPersister(indexerData.Registers, c.workingData.persistentRegisters, c.block.Height)
	if err := registerPersister.Persist(); err != nil {
		return fmt.Errorf("failed to persist registers: %w", err)
	}

	persisterStores := []stores.PersisterStore{
		stores.NewEventsStore(indexerData.Events, c.workingData.persistentEvents, c.executionResult.BlockID),
		stores.NewResultsStore(indexerData.Results, c.workingData.persistentResults, c.executionResult.BlockID),
		stores.NewCollectionsStore(indexerData.Collections, c.workingData.persistentCollections, c.workingData.lockManager),
		stores.NewTxResultErrMsgStore(c.workingData.txResultErrMsgsData, c.workingData.persistentTxResultErrMsgs, c.executionResult.BlockID),
		stores.NewLatestSealedResultStore(c.workingData.latestPersistedSealedResult, c.executionResult.ID(), c.block.Height),
	}
	blockPersister := persisters.NewBlockPersister(
		c.log,
		c.workingData.protocolDB,
		c.workingData.lockManager,
		c.executionResult,
		persisterStores,
	)
	if err := blockPersister.Persist(); err != nil {
		return fmt.Errorf("failed to persist block data: %w", err)
	}

	// reset the indexer data to prevent multiple calls to Persist
	c.workingData.indexerData = nil
	c.workingData.persisted = true

	return nil
}

// Abandon indicates that the protocol has abandoned this state. Hence processing will be aborted
// and any data dropped.
// This method will block until other in-progress operations are complete. If Download is in progress,
// the caller should cancel its context to ensure the operation completes in a timely manner.
//
// The method is idempotent. Calling it multiple times has no effect.
func (c *CoreImpl) Abandon() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.workingData = nil
}
