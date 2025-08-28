package optimistic_sync

import (
	"time"

	"github.com/onflow/flow-go/engine/access/ingestion/tx_error_messages"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/state_synchronization/requester"
	"github.com/onflow/flow-go/storage"
	"github.com/rs/zerolog"
)

// PipelineFactory is a factory object for creating new Pipeline instances.
type PipelineFactory interface {
	NewPipeline(result *flow.ExecutionResult, isSealed bool) Pipeline
}

type PipelineFactoryImpl struct {
	log           zerolog.Logger
	stateReceiver PipelineStateConsumer
}

func NewPipelineFactory(log zerolog.Logger, stateReceiver PipelineStateConsumer) PipelineFactory {
	return &PipelineFactoryImpl{
		log:           log,
		stateReceiver: stateReceiver,
	}
}

func (f *PipelineFactoryImpl) NewPipeline(result *flow.ExecutionResult, isSealed bool) Pipeline {
	return NewPipeline(f.log, result, isSealed, f.stateReceiver)
}

// CoreFactory is a factory object for creating new Core instances.
type CoreFactory interface {
	NewCore(result *flow.ExecutionResult, header *flow.Header) Core
}

type CoreFactoryImpl struct {
	log                           zerolog.Logger
	execDataRequester             requester.ExecutionDataRequester
	txResultErrMsgsRequester      tx_error_messages.Requester
	txResultErrMsgsRequestTimeout time.Duration
	persistentRegisters           storage.RegisterIndex
	persistentEvents              storage.Events
	persistentCollections         storage.Collections
	persistentTransactions        storage.Transactions
	persistentResults             storage.LightTransactionResults
	persistentTxResultErrMsg      storage.TransactionResultErrorMessages
	latestPersistedSealedResult   storage.LatestPersistedSealedResult
	protocolDB                    storage.DB
}

func NewCoreFactory(
	log zerolog.Logger,
	execDataRequester requester.ExecutionDataRequester,
	txResultErrMsgsRequester tx_error_messages.Requester,
	txResultErrMsgsRequestTimeout time.Duration,
	persistentRegisters storage.RegisterIndex,
	persistentEvents storage.Events,
	persistentCollections storage.Collections,
	persistentTransactions storage.Transactions,
	persistentResults storage.LightTransactionResults,
	persistentTxResultErrMsg storage.TransactionResultErrorMessages,
	latestPersistedSealedResult storage.LatestPersistedSealedResult,
	protocolDB storage.DB,
) CoreFactory {
	return &CoreFactoryImpl{
		log:                           log,
		execDataRequester:             execDataRequester,
		txResultErrMsgsRequester:      txResultErrMsgsRequester,
		txResultErrMsgsRequestTimeout: txResultErrMsgsRequestTimeout,
		persistentRegisters:           persistentRegisters,
		persistentEvents:              persistentEvents,
		persistentCollections:         persistentCollections,
		persistentTransactions:        persistentTransactions,
		persistentResults:             persistentResults,
		persistentTxResultErrMsg:      persistentTxResultErrMsg,
		latestPersistedSealedResult:   latestPersistedSealedResult,
		protocolDB:                    protocolDB,
	}
}

func (f *CoreFactoryImpl) NewCore(
	executionResult *flow.ExecutionResult,
	header *flow.Header,
) Core {
	return NewCoreImpl(
		f.log,
		executionResult,
		header,
		f.execDataRequester,
		f.txResultErrMsgsRequester,
		f.txResultErrMsgsRequestTimeout,
		f.persistentRegisters,
		f.persistentEvents,
		f.persistentCollections,
		f.persistentTransactions,
		f.persistentResults,
		f.persistentTxResultErrMsg,
		f.latestPersistedSealedResult,
		f.protocolDB,
	)
}
