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
) (*accessmodel.TransactionResult, accessmodel.ExecutorMetadata, error) {
	localResult, metadata, localErr := f.localProvider.TransactionResult(ctx, header, txID, collectionID, encodingVersion, executionResultInfo)
	if localErr == nil {
		return localResult, metadata, nil
	}

	return f.execNodeProvider.TransactionResult(ctx, header, txID, collectionID, encodingVersion, executionResultInfo)
}

func (f *FailoverTransactionProvider) TransactionResultByIndex(
	ctx context.Context,
	header *flow.Header,
	index uint32,
	collectionID flow.Identifier,
	encodingVersion entities.EventEncodingVersion,
	executionResultInfo *optimistic_sync.ExecutionResultInfo,
) (*accessmodel.TransactionResult, accessmodel.ExecutorMetadata, error) {
	localResult, metadata, localErr := f.localProvider.TransactionResultByIndex(ctx, header, index, collectionID, encodingVersion, executionResultInfo)
	if localErr == nil {
		return localResult, metadata, nil
	}

	return f.execNodeProvider.TransactionResultByIndex(ctx, header, index, collectionID, encodingVersion, executionResultInfo)
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
