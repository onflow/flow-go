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
	// TransactionResult retrieves a transaction result block ID and transaction ID.
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

	// TransactionResultsByBlockID retrieves a transaction result by block ID.
	//
	// Expected error returns during normal operation:
	//   - LocalProvider [optimistic_sync.ErrSnapshotNotFound] - when no snapshot is found for the execution result
	//   - ExecutionNodeProvider [storage.ErrNotFound] - when collection from block is not found
	//   - ExecutionNodeProvider [InvalidDataFromExternalNodeError] - when data returned by execution node is invalid
	//   - ExecutionNodeProvider [FailedToQueryExternalNodeError] - when the request to execution node failed
	TransactionResultsByBlockID(
		ctx context.Context,
		block *flow.Block,
		encodingVersion entities.EventEncodingVersion,
		executionResultInfo *optimistic_sync.ExecutionResultInfo,
	) ([]*accessmodel.TransactionResult, *accessmodel.ExecutorMetadata, error)

	TransactionsByBlockID(
		ctx context.Context,
		block *flow.Block,
		executionResultInfo *optimistic_sync.ExecutionResultInfo,
	) ([]*flow.TransactionBody, *accessmodel.ExecutorMetadata, error)

	SystemTransaction(
		ctx context.Context,
		block *flow.Block,
		txID flow.Identifier,
		executionResultInfo *optimistic_sync.ExecutionResultInfo,
	) (*flow.TransactionBody, *accessmodel.ExecutorMetadata, error)

	SystemTransactionResult(
		ctx context.Context,
		block *flow.Block,
		txID flow.Identifier,
		encodingVersion entities.EventEncodingVersion,
		executionResultInfo *optimistic_sync.ExecutionResultInfo,
	) (*accessmodel.TransactionResult, *accessmodel.ExecutorMetadata, error)
}
