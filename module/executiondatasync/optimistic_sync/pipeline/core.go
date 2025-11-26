package pipeline

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
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync/persisters"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync/persisters/stores"
	"github.com/onflow/flow-go/module/state_synchronization/indexer"
	"github.com/onflow/flow-go/module/state_synchronization/requester"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/store/inmemory/unsynchronized"
)

// ErrAbandoned is returned when the core is used after it has been abandoned.
var ErrAbandoned = errors.New("core is abandoned")

// TODO: DefaultTxResultErrMsgsRequestTimeout should be configured in future PR`s

const DefaultTxResultErrMsgsRequestTimeout = 5 * time.Second

// workingData encapsulates all components and temporary storage
// involved in processing a single block's execution data. When processing
// is complete or abandoned, the entire workingData can be discarded.
type workingData struct {
	// Temporary in-memory caches
	inmemRegisters       *unsynchronized.Registers
	inmemEvents          *unsynchronized.Events
	inmemCollections     *unsynchronized.Collections
	inmemTransactions    *unsynchronized.Transactions
	inmemResults         *unsynchronized.LightTransactionResults
	inmemTxResultErrMsgs *unsynchronized.TransactionResultErrorMessages

	// Active processing components
	execDataRequester             requester.ExecutionDataRequester
	txResultErrMsgsRequester      tx_error_messages.Requester
	txResultErrMsgsRequestTimeout time.Duration
	indexer                       *indexer.InMemoryIndexer
	blockPersister                *persisters.BlockPersister
	registersPersister            *persisters.RegistersPersister

	// Working data
	executionData       *execution_data.BlockExecutionDataEntity
	txResultErrMsgsData []flow.TransactionResultErrorMessage
}

var _ optimistic_sync.Core = (*Core)(nil)

// Core implements the core logic for execution data processing. It exposes methods for each of the
// processing steps, which must be called sequentially in the order:
// [Core.Download] ➔ [Core.Index] ➔ [Core.Persist].
// Abandon may be called at any time to abort processing and cleanup working data.
// The Core instance cannot be used after Abandon is called, and will return ErrAbandoned.
type Core struct {
	log zerolog.Logger
	mu  sync.Mutex

	workingData *workingData

	executionResult *flow.ExecutionResult
	header          *flow.Header
}

// NewCore creates a new Core with all necessary dependencies
func NewCore(
	logger zerolog.Logger,
	executionResult *flow.ExecutionResult,
	header *flow.Header,
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
) *Core {
	coreLogger := logger.With().
		Str("component", "execution_data_core").
		Str("execution_result_id", executionResult.ID().String()).
		Str("block_id", executionResult.BlockID.String()).
		Uint64("height", header.Height).
		Logger()

	inmemRegisters := unsynchronized.NewRegisters(header.Height)
	inmemEvents := unsynchronized.NewEvents()
	inmemTransactions := unsynchronized.NewTransactions()
	inmemCollections := unsynchronized.NewCollections(inmemTransactions)
	inmemResults := unsynchronized.NewLightTransactionResults()
	inmemTxResultErrMsgs := unsynchronized.NewTransactionResultErrorMessages()

	indexerComponent := indexer.NewInMemoryIndexer(
		coreLogger,
		inmemRegisters,
		inmemEvents,
		inmemCollections,
		inmemResults,
		inmemTxResultErrMsgs,
		executionResult,
		header,
		lockManager,
	)

	persisterStores := []stores.PersisterStore{
		stores.NewEventsStore(inmemEvents, persistentEvents, executionResult.BlockID),
		stores.NewResultsStore(inmemResults, persistentResults, executionResult.BlockID),
		stores.NewCollectionsStore(inmemCollections, persistentCollections, lockManager),
		stores.NewTxResultErrMsgStore(inmemTxResultErrMsgs, persistentTxResultErrMsg, executionResult.BlockID),
		stores.NewLatestSealedResultStore(latestPersistedSealedResult, executionResult.ID(), header.Height),
	}

	blockPersister := persisters.NewBlockPersister(
		coreLogger,
		protocolDB,
		lockManager,
		executionResult,
		header,
		persisterStores,
	)

	registerPersister := persisters.NewRegistersPersister(inmemRegisters, persistentRegisters, header.Height)

	return &Core{
		log: coreLogger,
		workingData: &workingData{
			execDataRequester:             execDataRequester,
			txResultErrMsgsRequester:      txResultErrMsgsRequester,
			txResultErrMsgsRequestTimeout: txResultErrMsgsRequestTimeout,
			indexer:                       indexerComponent,
			blockPersister:                blockPersister,
			registersPersister:            registerPersister,
			inmemRegisters:                inmemRegisters,
			inmemEvents:                   inmemEvents,
			inmemCollections:              inmemCollections,
			inmemTransactions:             inmemTransactions,
			inmemResults:                  inmemResults,
			inmemTxResultErrMsgs:          inmemTxResultErrMsgs,
		},
		executionResult: executionResult,
		header:          header,
	}
}

// Download requests the execution and transaction results data for the block.
//
// Expected error returns during normal operations:
// - [context.Canceled] if the provided context was canceled before completion
// - [ErrAbandoned] if processing this result has already been abandoned
func (c *Core) Download(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.workingData == nil {
		return ErrAbandoned
	}

	c.log.Debug().Msg("downloading execution data")
	g, gCtx := errgroup.WithContext(ctx)
	var executionData *execution_data.BlockExecutionData
	g.Go(func() error {
		var err error
		executionData, err = c.workingData.execDataRequester.RequestExecutionData(gCtx)
		//  executionData are CRITICAL. Any failure here causes the entire download to fail.
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
			// txResultErrMsgsData are OPTIONAL. Timeout error `context.DeadlineExceeded` is handled gracefully by
			// returning nil, allowing processing to continue with empty error messages data. Other errors still cause
			// failure.
			//
			// This approach ensures that temporary unavailability of transaction result error messages doesn't block
			// critical execution data processing.
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

	c.workingData.executionData = execution_data.NewBlockExecutionDataEntity(c.executionResult.ExecutionDataID, executionData)
	c.workingData.txResultErrMsgsData = txResultErrMsgsData

	c.log.Debug().Msg("successfully downloaded execution data")

	return nil
}

// Index retrieves the downloaded execution data and transaction results error messages from the
// caches and indexes them into in-memory storage.
//
// Expected error returns during normal operations:
// - [ErrAbandoned] if processing this result has already been abandoned
func (c *Core) Index() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.workingData == nil {
		return ErrAbandoned
	}

	if c.workingData.executionData == nil {
		return fmt.Errorf("could not index an empty execution data")
	}

	c.log.Debug().Msg("indexing execution data")

	if err := c.workingData.indexer.IndexBlockData(c.workingData.executionData); err != nil {
		return err
	}

	// Only index transaction result error messages when they are available
	if len(c.workingData.txResultErrMsgsData) > 0 {
		if err := c.workingData.indexer.IndexTxResultErrorMessagesData(c.workingData.txResultErrMsgsData); err != nil {
			return err
		}
	}

	c.log.Debug().Msg("successfully indexed execution data")

	return nil
}

// Persist persists the indexed data to permanent storage atomically.
//
// Expected error returns during normal operations:
// - [ErrAbandoned] if processing this result has already been abandoned
func (c *Core) Persist() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.workingData == nil {
		return ErrAbandoned
	}

	if c.workingData == nil {
		return fmt.Errorf("could not persist an empty execution data")
	}

	c.log.Debug().Msg("persisting execution data")

	if err := c.workingData.registersPersister.Persist(); err != nil {
		return fmt.Errorf("failed to persist register data: %w", err)
	}

	if err := c.workingData.blockPersister.Persist(); err != nil {
		return fmt.Errorf("failed to persist block data: %w", err)
	}

	c.log.Debug().Msg("successfully persisted execution data")

	return nil
}

// Abandon indicates that the protocol has abandoned this state. Hence processing will be aborted
// and any data dropped.
//
// Expected error returns during normal operations:
// - [ErrAbandoned] if processing this result has already been abandoned
func (c *Core) Abandon() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.workingData == nil {
		return ErrAbandoned
	}

	// Clear in-memory storage and other processing data by setting workingData references to nil for garbage collection
	c.workingData = nil

	return nil
}
