package pipeline

import (
	"context"
	"fmt"
	"github.com/onflow/flow-go/engine/access/ingestion/tx_error_messages"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
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
type CoreImpl struct {
	log zerolog.Logger

	execDataRequester        *requester.OneshotExecutionDataRequester
	txResultErrMsgsRequester *tx_error_messages.TransactionErrorMessagesRequester
	indexer                  *indexer.InMemoryIndexer
	persister                *indexer.Persister

	executionResult *flow.ExecutionResult
	header          *flow.Header

	registers       *unsynchronized.Registers
	events          *unsynchronized.Events
	collections     *unsynchronized.Collections
	transactions    *unsynchronized.Transactions
	results         *unsynchronized.LightTransactionResults
	txResultErrMsgs *unsynchronized.TransactionResultErrorMessages

	executionData       *execution_data.BlockExecutionDataEntity
	txResultErrMsgsData []flow.TransactionResultErrorMessage

	protocolDB storage.DB
}

// NewCoreImpl creates a new CoreImpl with all necessary dependencies
func NewCoreImpl(
	logger zerolog.Logger,
	executionResult *flow.ExecutionResult,
	header *flow.Header,
	downloader execution_data.Downloader,
	txErrCore *tx_error_messages.TxErrorMessagesCore,
	txErrMsgRequesterConfig tx_error_messages.TransactionErrorMessagesRequesterConfig,
	execDataRequesterConfig requester.OneshotExecutionDataConfig,
	execDataRequesterMetrics module.ExecutionDataRequesterMetrics,
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
		Logger()

	registers := unsynchronized.NewRegisters(header.Height)
	events := unsynchronized.NewEvents()
	collections := unsynchronized.NewCollections()
	transactions := unsynchronized.NewTransactions()
	results := unsynchronized.NewLightTransactionResults()
	txResultErrMsgs := unsynchronized.NewTransactionResultErrorMessages()

	indexerComponent := indexer.NewInMemoryIndexer(
		coreLogger,
		registers,
		events,
		collections,
		transactions,
		results,
		txResultErrMsgs,
		executionResult,
		header,
	)

	persisterComponent := indexer.NewPersister(
		coreLogger,
		registers,
		events,
		collections,
		transactions,
		results,
		txResultErrMsgs,
		persistentRegisters,
		persistentEvents,
		persistentCollections,
		persistentTransactions,
		persistentResults,
		persistentTxResultErrMsg,
		executionResult,
		header,
	)

	requesterComponent, err := requester.NewOneshotExecutionDataRequester(
		coreLogger,
		execDataRequesterMetrics,
		downloader,
		executionResult,
		header,
		execDataRequesterConfig,
	)
	if err != nil {
		return nil, err
	}

	return &CoreImpl{
		log:                      coreLogger,
		execDataRequester:        requesterComponent,
		txResultErrMsgsRequester: tx_error_messages.NewTransactionErrorMessagesRequester(txErrCore, &txErrMsgRequesterConfig, executionResult),
		indexer:                  indexerComponent,
		persister:                persisterComponent,
		executionResult:          executionResult,
		header:                   header,
		registers:                registers,
		events:                   events,
		collections:              collections,
		transactions:             transactions,
		results:                  results,
		txResultErrMsgs:          txResultErrMsgs,
		protocolDB:               protocolDB,
	}, nil
}

// Download implements the Core.Download method.
// It downloads execution data for the block and stores it in the execution data cache.
func (c *CoreImpl) Download(ctx context.Context) error {
	blockID := c.executionResult.BlockID
	height := c.header.Height

	c.log.Debug().
		Hex("block_id", logging.ID(blockID)).
		Uint64("height", height).
		Msg("downloading execution data")

	executionData, err := c.execDataRequester.RequestExecutionData(ctx)
	if err != nil {
		return err
	}

	c.executionData = execution_data.NewBlockExecutionDataEntity(c.executionResult.ExecutionDataID, executionData)

	//TODO: this should return []flow.TransactionResultErrorMessage and initialize txResultErrMsgsData
	err = c.txResultErrMsgsRequester.RequestTransactionErrorMessages(ctx)
	if err != nil {
		return err
	}

	//TODO: initialize txResultErrMsgsData with returned data from requester
	c.txResultErrMsgsData = make([]flow.TransactionResultErrorMessage, 0)

	return nil
}

// Index implements the Core.Index method.
// It retrieves the downloaded execution data from the cache and indexes it into in-memory storage.
func (c *CoreImpl) Index(ctx context.Context) error {
	blockID := c.executionResult.BlockID

	c.log.Debug().
		Hex("block_id", logging.ID(blockID)).
		Uint64("height", c.header.Height).
		Msg("indexing execution data")

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
func (c *CoreImpl) Persist(ctx context.Context) error {
	c.log.Debug().
		Hex("block_id", logging.ID(c.executionResult.BlockID)).
		Uint64("height", c.header.Height).
		Msg("persisting execution data")

	// Create a batch for atomic updates
	batch := c.protocolDB.NewBatch()
	defer func() {
		if err := batch.Close(); err != nil {
			c.log.Error().Err(err).Msg("failed to close batch")
		}
	}()

	// Add all data to the batch
	if err := c.persister.AddToBatch(batch); err != nil {
		return fmt.Errorf("failed to add data to batch: %w", err)
	}

	// Commit the batch
	if err := batch.Commit(); err != nil {
		return fmt.Errorf("failed to commit batch: %w", err)
	}

	c.log.Info().
		Hex("block_id", logging.ID(c.executionResult.BlockID)).
		Uint64("height", c.header.Height).
		Msg("successfully persisted execution data")

	return nil
}

// Abandon indicates that the protocol has abandoned this state. Hence processing will be aborted
// and any data dropped.
func (c *CoreImpl) Abandon(ctx context.Context) error {
	c.log.Debug().
		Hex("block_id", logging.ID(c.executionResult.BlockID)).
		Uint64("height", c.header.Height).
		Msg("Abandon execution data processing")

	// Clear in-memory storage by setting references to nil for garbage collection
	// Since we don't have Clear() methods, we remove the references to allow GC
	c.registers = nil
	c.events = nil
	c.collections = nil
	c.transactions = nil
	c.results = nil
	c.txResultErrMsgs = nil

	// Clear other references
	c.txResultErrMsgsRequester = nil
	c.execDataRequester = nil
	c.indexer = nil
	c.persister = nil

	return nil
}
