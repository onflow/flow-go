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
	encodingVersion entities.EventEncodingVersion,
	executionResultInfo *optimistic_sync.ExecutionResultInfo,
) (*accessmodel.TransactionResult, accessmodel.ExecutorMetadata, error) {
	localResult, metadata, localErr := f.localProvider.TransactionResult(ctx, header, txID, encodingVersion, executionResultInfo)
	if localErr == nil {
		return localResult, metadata, nil
	}

	return f.execNodeProvider.TransactionResult(ctx, header, txID, encodingVersion, executionResultInfo)
}

func (f *FailoverTransactionProvider) TransactionResultByIndex(
	ctx context.Context,
	block *flow.Block,
	index uint32,
	encodingVersion entities.EventEncodingVersion,
	executionResultInfo *optimistic_sync.ExecutionResultInfo,
) (*accessmodel.TransactionResult, accessmodel.ExecutorMetadata, error) {
	localResult, metadata, localErr := f.localProvider.TransactionResultByIndex(ctx, block, index, encodingVersion, executionResultInfo)
	if localErr == nil {
		return localResult, metadata, nil
	}

	return f.execNodeProvider.TransactionResultByIndex(ctx, block, index, encodingVersion, executionResultInfo)
}

func (f *FailoverTransactionProvider) TransactionResultsByBlockID(
	ctx context.Context,
	block *flow.Block,
	encodingVersion entities.EventEncodingVersion,
	executionResultInfo *optimistic_sync.ExecutionResultInfo,
) ([]*accessmodel.TransactionResult, accessmodel.ExecutorMetadata, error) {
	localResults, metadata, localErr := f.localProvider.TransactionResultsByBlockID(ctx, block, encodingVersion, executionResultInfo)
	if localErr == nil {
		return localResults, metadata, nil
	}

	return f.execNodeProvider.TransactionResultsByBlockID(ctx, block, encodingVersion, executionResultInfo)
}
