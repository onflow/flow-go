package provider

import (
	"context"

	"github.com/onflow/flow/protobuf/go/flow/entities"

	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
)

type FailoverTransactionProvider struct {
	localProvider    TransactionProvider
	execNodeProvider TransactionProvider
}

var _ TransactionProvider = (*FailoverTransactionProvider)(nil)

func NewFailoverTransactionProvider(local TransactionProvider, execNode TransactionProvider) *FailoverTransactionProvider {
	return &FailoverTransactionProvider{
		localProvider:    local,
		execNodeProvider: execNode,
	}
}

// TransactionResult retrieves a transaction result by block ID and transaction ID, by first querying
// the local provider and then the execution node provider if the local provider fails.
//
// Expected error returns during normal operation:
//   - LocalProvider [optimistic_sync.ErrSnapshotNotFound] - when no snapshot is found for the execution result
//   - ExecutionNodeProvider [InvalidDataFromExternalNodeError] - when data returned by execution node is invalid
//   - ExecutionNodeProvider [FailedToQueryExternalNodeError] - when the request to execution node failed
func (f *FailoverTransactionProvider) TransactionResult(
	ctx context.Context,
	header *flow.Header,
	txID flow.Identifier,
	collectionID flow.Identifier,
	encodingVersion entities.EventEncodingVersion,
	executionResultInfo *optimistic_sync.ExecutionResultInfo,
) (*accessmodel.TransactionResult, *accessmodel.ExecutorMetadata, error) {
	localResult, metadata, localErr := f.localProvider.TransactionResult(ctx, header, txID, collectionID, encodingVersion, executionResultInfo)
	if localErr == nil {
		return localResult, metadata, nil
	}

	return f.execNodeProvider.TransactionResult(ctx, header, txID, collectionID, encodingVersion, executionResultInfo)
}

// TransactionResultByIndex retrieves a transaction result by block ID and index, by first querying
// the local provider and then the execution node provider if the local provider fails.
//
// Expected error returns during normal operation:
//   - LocalProvider [optimistic_sync.ErrSnapshotNotFound] - when no snapshot is found for the execution result
//   - ExecutionNodeProvider [InvalidDataFromExternalNodeError] - when data returned by execution node is invalid
//   - ExecutionNodeProvider [FailedToQueryExternalNodeError] - when the request to execution node failed
func (f *FailoverTransactionProvider) TransactionResultByIndex(
	ctx context.Context,
	header *flow.Header,
	index uint32,
	collectionID flow.Identifier,
	encodingVersion entities.EventEncodingVersion,
	executionResultInfo *optimistic_sync.ExecutionResultInfo,
) (*accessmodel.TransactionResult, *accessmodel.ExecutorMetadata, error) {
	localResult, metadata, localErr := f.localProvider.TransactionResultByIndex(ctx, header, index, collectionID, encodingVersion, executionResultInfo)
	if localErr == nil {
		return localResult, metadata, nil
	}

	return f.execNodeProvider.TransactionResultByIndex(ctx, header, index, collectionID, encodingVersion, executionResultInfo)
}

// TransactionResultsByBlockID retrieves all transaction results for a block by block ID, by first
// querying the local provider and then the execution node provider if the local provider fails.
//
// Expected error returns during normal operation:
//   - All providers [ErrNotASystemTransaction] - when the requested transaction is not a system transaction
//   - LocalProvider [optimistic_sync.ErrSnapshotNotFound] - when no snapshot is found for the execution result
//   - ExecutionNodeProvider [common.InvalidDataFromExternalNodeError] - when data returned by execution node is invalid
//   - ExecutionNodeProvider [common.FailedToQueryExternalNodeError] - when the request to execution node failed
func (f *FailoverTransactionProvider) TransactionResultsByBlockID(
	ctx context.Context,
	block *flow.Block,
	encodingVersion entities.EventEncodingVersion,
	executionResultInfo *optimistic_sync.ExecutionResultInfo,
) ([]*accessmodel.TransactionResult, *accessmodel.ExecutorMetadata, error) {
	localResults, metadata, localErr := f.localProvider.TransactionResultsByBlockID(ctx, block, encodingVersion, executionResultInfo)
	if localErr == nil {
		return localResults, metadata, nil
	}

	return f.execNodeProvider.TransactionResultsByBlockID(ctx, block, encodingVersion, executionResultInfo)
}

// TransactionsByBlockID retrieves all transactions for a block by block ID, by first querying
// the local provider and then the execution node provider if the local provider fails.
//
// Expected error returns during normal operation:
//   - LocalProvider [optimistic_sync.ErrSnapshotNotFound] - when no snapshot is found for the execution result
//   - ExecutionNodeProvider [ErrBlockCollectionNotFound] - when a collection from the block is not found
//   - ExecutionNodeProvider [NewIncorrectResultCountError] - when the number of transaction results returned by the execution
//     node is less than the number of transactions in the block
//   - ExecutionNodeProvider [common.InvalidDataFromExternalNodeError] - when data returned by execution node is invalid
//   - ExecutionNodeProvider [common.FailedToQueryExternalNodeError] - when the request to execution node failed
func (f *FailoverTransactionProvider) TransactionsByBlockID(
	ctx context.Context,
	block *flow.Block,
	executionResultInfo *optimistic_sync.ExecutionResultInfo,
) ([]*flow.TransactionBody, *accessmodel.ExecutorMetadata, error) {
	localResults, metadata, localErr := f.localProvider.TransactionsByBlockID(ctx, block, executionResultInfo)
	if localErr == nil {
		return localResults, metadata, nil
	}

	return f.execNodeProvider.TransactionsByBlockID(ctx, block, executionResultInfo)
}

// SystemTransaction retrieves a system transaction for a block by block ID and transaction ID, by
// first querying the local provider and then the execution node provider if the local provider fails.
//
// Expected error returns during normal operation:
//   - All providers [ErrNotASystemTransaction] - when the requested transaction is not a system transaction
//   - LocalProvider [optimistic_sync.ErrSnapshotNotFound] - when no snapshot is found for the execution result
//   - ExecutionNodeProvider [common.InvalidDataFromExternalNodeError] when events data returned by execution node is invalid
//   - ExecutionNodeProvider [common.FailedToQueryExternalNodeError] when the request to execution node failed
func (f *FailoverTransactionProvider) SystemTransaction(
	ctx context.Context,
	block *flow.Block,
	txID flow.Identifier,
	executionResultInfo *optimistic_sync.ExecutionResultInfo,
) (*flow.TransactionBody, *accessmodel.ExecutorMetadata, error) {
	localResult, metadata, localErr := f.localProvider.SystemTransaction(ctx, block, txID, executionResultInfo)
	if localErr == nil {
		return localResult, metadata, nil
	}

	return f.execNodeProvider.SystemTransaction(ctx, block, txID, executionResultInfo)
}

// SystemTransactionResult retrieves a system transaction result for a block by block ID and transaction
// ID, by first querying the local provider and then the execution node provider if the local provider
// fails.
//
// Expected error returns during normal operation:
//   - All providers [ErrNotASystemTransaction] - when the requested transaction is not a system transaction
//   - LocalProvider [optimistic_sync.ErrSnapshotNotFound] - when no snapshot is found for the execution result
//   - ExecutionNodeProvider [common.InvalidDataFromExternalNodeError] - when transaction result data returned by execution node is invalid
//   - ExecutionNodeProvider [common.FailedToQueryExternalNodeError] - when the request to execution node failed
func (f *FailoverTransactionProvider) SystemTransactionResult(
	ctx context.Context,
	block *flow.Block,
	txID flow.Identifier,
	encodingVersion entities.EventEncodingVersion,
	executionResultInfo *optimistic_sync.ExecutionResultInfo,
) (*accessmodel.TransactionResult, *accessmodel.ExecutorMetadata, error) {
	localResult, metadata, localErr := f.localProvider.SystemTransactionResult(ctx, block, txID, encodingVersion, executionResultInfo)
	if localErr == nil {
		return localResult, metadata, nil
	}

	return f.execNodeProvider.SystemTransactionResult(ctx, block, txID, encodingVersion, executionResultInfo)
}
