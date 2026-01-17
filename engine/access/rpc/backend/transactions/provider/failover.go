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

func (f *FailoverTransactionProvider) TransactionResult(
	ctx context.Context,
	header *flow.Header,
	txID flow.Identifier,
	collectionID flow.Identifier,
	encodingVersion entities.EventEncodingVersion,
	executionResultInfo *optimistic_sync.ExecutionResultInfo,
) (*accessmodel.TransactionResult, *accessmodel.ExecutorMetadata, error) {
	localResult, localMetadata, localErr := f.localProvider.TransactionResult(ctx, header, txID, collectionID, encodingVersion, executionResultInfo)
	if localErr == nil {
		return localResult, localMetadata, nil
	}

	return f.execNodeProvider.TransactionResult(ctx, header, txID, collectionID, encodingVersion, executionResultInfo)
}

func (f *FailoverTransactionProvider) TransactionResultByIndex(
	ctx context.Context,
	block *flow.Block,
	index uint32,
	collectionID flow.Identifier,
	encodingVersion entities.EventEncodingVersion,
	executionResultInfo *optimistic_sync.ExecutionResultInfo,
) (*accessmodel.TransactionResult, *accessmodel.ExecutorMetadata, error) {
	localResult, localMetadata, localErr := f.localProvider.TransactionResultByIndex(ctx, block, index, collectionID, encodingVersion, executionResultInfo)
	if localErr == nil {
		return localResult, localMetadata, nil
	}

	return f.execNodeProvider.TransactionResultByIndex(ctx, block, index, collectionID, encodingVersion, executionResultInfo)
}

func (f *FailoverTransactionProvider) TransactionResultsByBlockID(
	ctx context.Context,
	block *flow.Block,
	encodingVersion entities.EventEncodingVersion,
	executionResultInfo *optimistic_sync.ExecutionResultInfo,
) ([]*accessmodel.TransactionResult, *accessmodel.ExecutorMetadata, error) {
	localResults, localMetadata, localErr := f.localProvider.TransactionResultsByBlockID(ctx, block, encodingVersion, executionResultInfo)
	if localErr == nil {
		return localResults, localMetadata, nil
	}

	return f.execNodeProvider.TransactionResultsByBlockID(ctx, block, encodingVersion, executionResultInfo)
}

func (f *FailoverTransactionProvider) TransactionsByBlockID(
	ctx context.Context,
	block *flow.Block,
) ([]*flow.TransactionBody, error) {
	localResults, localErr := f.localProvider.TransactionsByBlockID(ctx, block)
	if localErr == nil {
		return localResults, nil
	}

	return f.execNodeProvider.TransactionsByBlockID(ctx, block)
}

func (f *FailoverTransactionProvider) ScheduledTransactionsByBlockID(
	ctx context.Context,
	header *flow.Header,
) ([]*flow.TransactionBody, error) {
	localResults, localErr := f.localProvider.ScheduledTransactionsByBlockID(ctx, header)
	if localErr == nil {
		return localResults, nil
	}

	return f.execNodeProvider.ScheduledTransactionsByBlockID(ctx, header)
}
