package provider

import (
	"context"

	"github.com/onflow/flow/protobuf/go/flow/entities"

	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
)

// TransactionProvider defines an interface for retrieving transaction results
// from various data sources, such as local storage and execution nodes.
type TransactionProvider interface {
	// TransactionResult retrieves a transaction result by block ID and transaction ID.
	//
	// Expected error returns during normal operation:
	//   - LocalProvider [optimistic_sync.ErrSnapshotNotFound] - when no snapshot is found for the execution result
	//   - ExecutionNodeProvider [InvalidDataFromExternalNodeError] - when data returned by execution node is invalid
	//   - ExecutionNodeProvider [FailedToQueryExternalNodeError] - when the request to execution node failed
	TransactionResult(
		ctx context.Context,
		header *flow.Header,
		txID flow.Identifier,
		collectionID flow.Identifier,
		encodingVersion entities.EventEncodingVersion,
		executionResultInfo *optimistic_sync.ExecutionResultInfo,
	) (*accessmodel.TransactionResult, *accessmodel.ExecutorMetadata, error)

	// TransactionResultByIndex retrieves a transaction result by block ID and index.
	//
	// Expected error returns during normal operation:
	//   - LocalProvider [optimistic_sync.ErrSnapshotNotFound] - when no snapshot is found for the execution result
	//   - ExecutionNodeProvider [InvalidDataFromExternalNodeError] - when data returned by execution node is invalid
	//   - ExecutionNodeProvider [FailedToQueryExternalNodeError] - when the request to execution node failed
	TransactionResultByIndex(
		ctx context.Context,
		header *flow.Header,
		index uint32,
		collectionID flow.Identifier,
		encodingVersion entities.EventEncodingVersion,
		executionResultInfo *optimistic_sync.ExecutionResultInfo,
	) (*accessmodel.TransactionResult, *accessmodel.ExecutorMetadata, error)

	// TransactionResultsByBlockID retrieves all transaction results for a block by block ID.
	//
	// Expected error returns during normal operation:
	//   - All providers [ErrNotASystemTransaction] - when the requested transaction is not a system transaction
	//   - LocalProvider [optimistic_sync.ErrSnapshotNotFound] - when no snapshot is found for the execution result
	//   - ExecutionNodeProvider [common.InvalidDataFromExternalNodeError] - when data returned by execution node is invalid
	//   - ExecutionNodeProvider [common.FailedToQueryExternalNodeError] - when the request to execution node failed
	TransactionResultsByBlockID(
		ctx context.Context,
		block *flow.Block,
		encodingVersion entities.EventEncodingVersion,
		executionResultInfo *optimistic_sync.ExecutionResultInfo,
	) ([]*accessmodel.TransactionResult, *accessmodel.ExecutorMetadata, error)

	// TransactionsByBlockID retrieves all transactions for a block by block ID.
	//
	// Expected error returns during normal operation:
	//   - LocalProvider [optimistic_sync.ErrSnapshotNotFound] - when no snapshot is found for the execution result
	//   - ExecutionNodeProvider [ErrBlockCollectionNotFound] - when a collection from the block is not found
	//   - ExecutionNodeProvider [NewIncorrectResultCountError] - when the number of transaction results returned by the execution
	//     node is less than the number of transactions in the block
	//   - ExecutionNodeProvider [common.InvalidDataFromExternalNodeError] - when data returned by execution node is invalid
	//   - ExecutionNodeProvider [common.FailedToQueryExternalNodeError] - when the request to execution node failed
	TransactionsByBlockID(
		ctx context.Context,
		block *flow.Block,
		executionResultInfo *optimistic_sync.ExecutionResultInfo,
	) ([]*flow.TransactionBody, *accessmodel.ExecutorMetadata, error)

	// SystemTransaction retrieves a system transaction for a block by block ID and transaction ID.
	//
	// Expected error returns during normal operation:
	//   - All providers [ErrNotASystemTransaction] - when the requested transaction is not a system transaction
	//   - LocalProvider [optimistic_sync.ErrSnapshotNotFound] - when no snapshot is found for the execution result
	//   - ExecutionNodeProvider [common.InvalidDataFromExternalNodeError] when events data returned by execution node is invalid
	//   - ExecutionNodeProvider [common.FailedToQueryExternalNodeError] when the request to execution node failed
	SystemTransaction(
		ctx context.Context,
		block *flow.Block,
		txID flow.Identifier,
		executionResultInfo *optimistic_sync.ExecutionResultInfo,
	) (*flow.TransactionBody, *accessmodel.ExecutorMetadata, error)

	// SystemTransactionResult retrieves a system transaction result for a block by block ID and transaction ID.
	//
	// Expected error returns during normal operation:
	//   - All providers [ErrNotASystemTransaction] - when the requested transaction is not a system transaction
	//   - LocalProvider [optimistic_sync.ErrSnapshotNotFound] - when no snapshot is found for the execution result
	//   - ExecutionNodeProvider [common.InvalidDataFromExternalNodeError] - when transaction result data returned by execution node is invalid
	//   - ExecutionNodeProvider [common.FailedToQueryExternalNodeError] - when the request to execution node failed
	SystemTransactionResult(
		ctx context.Context,
		block *flow.Block,
		txID flow.Identifier,
		encodingVersion entities.EventEncodingVersion,
		executionResultInfo *optimistic_sync.ExecutionResultInfo,
	) (*accessmodel.TransactionResult, *accessmodel.ExecutorMetadata, error)
}
