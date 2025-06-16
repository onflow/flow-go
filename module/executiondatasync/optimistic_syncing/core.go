package pipeline

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/access/ingestion/tx_error_messages"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/state_synchronization/indexer"
	"github.com/onflow/flow-go/module/state_synchronization/requester"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/store/inmemory/unsynchronized"
	"github.com/onflow/flow-go/utils/logging"
)

// Core defines the interface for pipeline processing steps.
// Each implementation should handle an execution data and implement the three-phase processing:
// download, index, and persist.
type Core interface {
	// Download retrieves all necessary data for processing.
	Download(ctx context.Context) error

	// Index processes the downloaded data and creates in-memory indexes.
	Index(ctx context.Context) error

	// Persist stores the indexed data in permanent storage.
	Persist(ctx context.Context) error

	// Abandon indicates that the protocol has abandoned this state. Hence processing will be aborted
	// and any data dropped.
	Abandon(ctx context.Context) error
}

var _ Core = (*CoreImpl)(nil)

// CoreImpl implements the Core interface for processing execution data.
// It coordinates the download, indexing, and persisting of execution data.
// CoreImpl is not safe for concurrent use. Use only within a single gorountine.
type CoreImpl struct {
	log zerolog.Logger

	execDataRequester        requester.ExecutionDataRequester
	txResultErrMsgsRequester tx_error_messages.TransactionResultErrorMessageRequester
	indexer                  *indexer.InMemoryIndexer
	persister                *indexer.Persister

	executionResult *flow.ExecutionResult
	header          *flow.Header

	inmemRegisters       *unsynchronized.Registers
	inmemEvents          *unsynchronized.Events
	inmemCollections     *unsynchronized.Collections
	inmemTransactions    *unsynchronized.Transactions
	inmemResults         *unsynchronized.LightTransactionResults
	inmemTxResultErrMsgs *unsynchronized.TransactionResultErrorMessages

	executionData       *execution_data.BlockExecutionDataEntity
	txResultErrMsgsData []flow.TransactionResultErrorMessage
}

// NewCoreImpl creates a new CoreImpl with all necessary dependencies
func NewCoreImpl(
	logger zerolog.Logger,
	executionResult *flow.ExecutionResult,
	header *flow.Header,
	execDataRequester requester.ExecutionDataRequester,
	txResultErrMsgsRequester tx_error_messages.TransactionResultErrorMessageRequester,
	persistentRegisters storage.RegisterIndex,
	persistentEvents storage.Events,
	persistentCollections storage.Collections,
	persistentTransactions storage.Transactions,
	persistentResults storage.LightTransactionResults,
	persistentTxResultErrMsg storage.TransactionResultErrorMessages,
	protocolDB storage.DB,
) (*CoreImpl, error) {
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
		log:                      coreLogger,
		execDataRequester:        execDataRequester,
		txResultErrMsgsRequester: txResultErrMsgsRequester,
		indexer:                  indexerComponent,
		persister:                persisterComponent,
		executionResult:          executionResult,
		header:                   header,
		inmemRegisters:           inmemRegisters,
		inmemEvents:              inmemEvents,
		inmemCollections:         inmemCollections,
		inmemTransactions:        inmemTransactions,
		inmemResults:             inmemResults,
		inmemTxResultErrMsgs:     inmemTxResultErrMsgs,
	}, nil
}

// Download implements the Core.Download method.
// It downloads execution data for the block and stores it in the execution data cache.
// Expected errors:
// - context.Canceled: if the provided context was canceled before completion
// All other errors are unexpected exceptions.
func (c *CoreImpl) Download(ctx context.Context) error {
	blockID := c.executionResult.BlockID
	height := c.header.Height

	c.log.Debug().
		Hex("block_id", logging.ID(blockID)).
		Uint64("height", height).
		Msg("downloading execution data")

	executionData, err := c.execDataRequester.RequestExecutionData(ctx)
	if err != nil {
		return fmt.Errorf("failed to request execution data: %w", err)
	}

	c.executionData = execution_data.NewBlockExecutionDataEntity(c.executionResult.ExecutionDataID, executionData)

	txResultErrMsgsData, err := c.txResultErrMsgsRequester.Request(ctx)
	if err != nil {
		return fmt.Errorf("failed to request transaction result error messages data: %w", err)
	}

	c.txResultErrMsgsData = txResultErrMsgsData

	return nil
}

// Index implements the Core.Index method.
// It retrieves the downloaded execution data from the cache and indexes it into in-memory storage.
// Expected errors:
// - convert.UnexpectedLedgerKeyFormat if the key is not in the expected format
// - storage.ErrHeightNotIndexed if the given height does not match the storage's block height.
//
// All other errors are unexpected exceptions.
func (c *CoreImpl) Index(_ context.Context) error {
	c.log.Debug().Msg("indexing execution data")

	if err := c.indexer.IndexBlockData(c.executionData); err != nil {
		return err
	}

	if err := c.indexer.IndexTxResultErrorMessagesData(c.txResultErrMsgsData); err != nil {
		return err
	}

	return nil
}

// Persist implements the Core.Persist method.
// It persists the indexed data to permanent storage atomically.
//   - ErrRecordExists if the register record already exists
//
// All other errors are unexpected exceptions.
func (c *CoreImpl) Persist(_ context.Context) error {
	c.log.Debug().Msg("persisting execution data")

	// Add all data to the batch
	if err := c.persister.Persist(); err != nil {
		return fmt.Errorf("failed to persist data: %w", err)
	}

	c.log.Info().Msg("successfully persisted execution data")

	return nil
}

// Abandon indicates that the protocol has abandoned this state. Hence processing will be aborted
// and any data dropped.
func (c *CoreImpl) Abandon(_ context.Context) error {
	c.log.Debug().Msg("Abandon execution data processing")

	// Clear in-memory storage by setting references to nil for garbage collection
	// Since we don't have Clear() methods, we remove the references to allow GC
	c.inmemRegisters = nil
	c.inmemEvents = nil
	c.inmemCollections = nil
	c.inmemTransactions = nil
	c.inmemResults = nil
	c.inmemTxResultErrMsgs = nil

	// Clear other references
	c.txResultErrMsgsRequester = nil
	c.execDataRequester = nil
	c.indexer = nil
	c.persister = nil
	c.executionData = nil
	c.txResultErrMsgsData = nil

	return nil
}
