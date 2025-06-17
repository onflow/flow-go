package pipeline

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"

	"github.com/onflow/flow-go/engine/access/ingestion/tx_error_messages"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/state_synchronization/indexer"
	"github.com/onflow/flow-go/module/state_synchronization/requester"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/store/inmemory/unsynchronized"
)

// TODO: DefaultTxResultErrMsgsRequestTimeout should be configured in future PR`s

const DefaultTxResultErrMsgsRequestTimeout = 5 * time.Second

// Core defines the interface for pipeline processing steps.
// Each implementation should handle an execution data and implement the three-phase processing:
// download, index, and persist.
type Core interface {
	// Download retrieves all necessary data for processing.
	// Expected errors:
	// - context.Canceled: if the provided context was canceled before completion
	//
	// No other errors are expected during normal operations
	Download(ctx context.Context) error

	// Index processes the downloaded data and creates in-memory indexes.
	//
	// No errors are expected during normal operations
	Index() error

	// Persist stores the indexed data in permanent storage.
	//
	// No errors are expected during normal operations
	Persist() error

	// Abandon indicates that the protocol has abandoned this state. Hence processing will be aborted
	// and any data dropped.
	//
	// No errors are expected during normal operations
	Abandon() error
}

// executionDataProcessor encapsulates all components and temporary storage
// involved in processing a single block's execution data. When processing
// is complete or abandoned, the entire processor can be discarded.
type executionDataProcessor struct {
	// Temporary in-memory caches
	inmemRegisters       *unsynchronized.Registers
	inmemEvents          *unsynchronized.Events
	inmemCollections     *unsynchronized.Collections
	inmemTransactions    *unsynchronized.Transactions
	inmemResults         *unsynchronized.LightTransactionResults
	inmemTxResultErrMsgs *unsynchronized.TransactionResultErrorMessages

	// Active processing components
	execDataRequester             requester.ExecutionDataRequester
	txResultErrMsgsRequester      tx_error_messages.TransactionResultErrorMessageRequester
	txResultErrMsgsRequestTimeout time.Duration
	indexer                       *indexer.InMemoryIndexer
	persister                     *indexer.Persister

	// Working data
	executionData       *execution_data.BlockExecutionDataEntity
	txResultErrMsgsData []flow.TransactionResultErrorMessage
}

var _ Core = (*CoreImpl)(nil)

// CoreImpl implements the Core interface for processing execution data.
// It coordinates the download, indexing, and persisting of execution data.
// CoreImpl is not safe for concurrent use. Use only within a single gorountine.
type CoreImpl struct {
	log zerolog.Logger

	processor *executionDataProcessor

	executionResult *flow.ExecutionResult
	header          *flow.Header
}

// NewCoreImpl creates a new CoreImpl with all necessary dependencies
func NewCoreImpl(
	logger zerolog.Logger,
	executionResult *flow.ExecutionResult,
	header *flow.Header,
	execDataRequester requester.ExecutionDataRequester,
	txResultErrMsgsRequester tx_error_messages.TransactionResultErrorMessageRequester,
	txResultErrMsgsRequestTimeout time.Duration,
	persistentRegisters storage.RegisterIndex,
	persistentEvents storage.Events,
	persistentCollections storage.Collections,
	persistentTransactions storage.Transactions,
	persistentResults storage.LightTransactionResults,
	persistentTxResultErrMsg storage.TransactionResultErrorMessages,
	protocolDB storage.DB,
) *CoreImpl {
	coreLogger := logger.With().
		Str("component", "execution_data_core").
		Str("execution_result_id", executionResult.ID().String()).
		Str("block_id", executionResult.BlockID.String()).
		Uint64("height", header.Height).
		Logger()

	inmemRegisters := unsynchronized.NewRegisters(header.Height)
	inmemEvents := unsynchronized.NewEvents()
	inmemCollections := unsynchronized.NewCollections()
	inmemTransactions := unsynchronized.NewTransactions()
	inmemResults := unsynchronized.NewLightTransactionResults()
	inmemTxResultErrMsgs := unsynchronized.NewTransactionResultErrorMessages()

	indexerComponent := indexer.NewInMemoryIndexer(
		coreLogger,
		inmemRegisters,
		inmemEvents,
		inmemCollections,
		inmemTransactions,
		inmemResults,
		inmemTxResultErrMsgs,
		executionResult,
		header,
	)

	persisterComponent := indexer.NewPersister(
		coreLogger,
		inmemRegisters,
		inmemEvents,
		inmemCollections,
		inmemTransactions,
		inmemResults,
		inmemTxResultErrMsgs,
		persistentRegisters,
		persistentEvents,
		persistentCollections,
		persistentTransactions,
		persistentResults,
		persistentTxResultErrMsg,
		protocolDB,
		executionResult,
		header,
	)

	return &CoreImpl{
		log: coreLogger,
		processor: &executionDataProcessor{
			execDataRequester:             execDataRequester,
			txResultErrMsgsRequester:      txResultErrMsgsRequester,
			txResultErrMsgsRequestTimeout: txResultErrMsgsRequestTimeout,
			indexer:                       indexerComponent,
			persister:                     persisterComponent,
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

// Download downloads execution data and transaction results error for the block
// Expected errors:
// - context.Canceled: if the provided context was canceled before completion
// - context.DeadlineExceeded: if the provided context was canceled due to its deadline reached
//
// No other errors are expected during normal operations
func (c *CoreImpl) Download(ctx context.Context) error {
	c.log.Debug().Msg("downloading execution data")

	g, gCtx := errgroup.WithContext(ctx)

	var executionData *execution_data.BlockExecutionData
	g.Go(func() error {
		var err error
		executionData, err = c.processor.execDataRequester.RequestExecutionData(gCtx)
		if err != nil {
			return fmt.Errorf("failed to request execution data: %w", err)
		}

		return nil
	})

	var txResultErrMsgsData []flow.TransactionResultErrorMessage
	g.Go(func() error {
		timeoutCtx, cancel := context.WithTimeout(gCtx, c.processor.txResultErrMsgsRequestTimeout)
		defer cancel()

		var err error
		txResultErrMsgsData, err = c.processor.txResultErrMsgsRequester.Request(timeoutCtx)
		if err != nil {
			return fmt.Errorf("failed to request transaction result error messages data: %w", err)
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		//TODO: For me executionData and txResultErrMsgsData are not equally important. Could we proceed indexing without `txResultErrMsgsData`?
		return err
	}

	c.processor.executionData = execution_data.NewBlockExecutionDataEntity(c.executionResult.ExecutionDataID, executionData)
	c.processor.txResultErrMsgsData = txResultErrMsgsData

	c.log.Debug().Msg("successfully downloaded execution data")

	return nil
}

// Index retrieves the downloaded execution data and transaction results error messages from the caches and indexes them
// into in-memory storage.
//
// No errors are expected during normal operations
func (c *CoreImpl) Index() error {
	if c.processor.executionData == nil {
		return fmt.Errorf("could not index an empty execution data")
	}
	c.log.Debug().Msg("indexing execution data")

	if err := c.processor.indexer.IndexBlockData(c.processor.executionData); err != nil {
		return err
	}

	if err := c.processor.indexer.IndexTxResultErrorMessagesData(c.processor.txResultErrMsgsData); err != nil {
		return err
	}

	c.log.Debug().Msg("successfully indexed execution data")

	return nil
}

// Persist persists the indexed data to permanent storage atomically.
//
// No errors are expected during normal operations
func (c *CoreImpl) Persist() error {
	c.log.Debug().Msg("persisting execution data")

	// Add all data to the batch
	if err := c.processor.persister.Persist(); err != nil {
		return fmt.Errorf("failed to persist data: %w", err)
	}

	c.log.Debug().Msg("successfully persisted execution data")

	return nil
}

// Abandon indicates that the protocol has abandoned this state. Hence processing will be aborted
// and any data dropped.
//
// No errors are expected during normal operations
func (c *CoreImpl) Abandon() error {
	// Clear in-memory storage and other processing data by setting processor references to nil for garbage collection
	c.processor = nil

	return nil
}
