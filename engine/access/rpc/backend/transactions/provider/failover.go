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
	criteria optimistic_sync.Criteria,
) (*accessmodel.TransactionResult, *optimistic_sync.ExecutionResultInfo, error) {
	localResult, executionResultInfo, localErr := f.localProvider.TransactionResult(ctx, header, txID,
		encodingVersion, criteria)
	if localErr == nil {
		return localResult, executionResultInfo, nil
	}

	execNodeResult, executionResultInfo, execNodeErr := f.execNodeProvider.TransactionResult(ctx, header, txID,
		encodingVersion, criteria)
	return execNodeResult, executionResultInfo, execNodeErr
}

func (f *FailoverTransactionProvider) TransactionResultByIndex(
	ctx context.Context,
	block *flow.Block,
	index uint32,
	encodingVersion entities.EventEncodingVersion,
	criteria optimistic_sync.Criteria,
) (*accessmodel.TransactionResult, *optimistic_sync.ExecutionResultInfo, error) {
	localResult, executionResultInfo, localErr := f.localProvider.TransactionResultByIndex(ctx, block, index,
		encodingVersion, criteria)
	if localErr == nil {
		return localResult, executionResultInfo, nil
	}

	execNodeResult, executionResultInfo, execNodeErr := f.execNodeProvider.TransactionResultByIndex(ctx, block, index,
		encodingVersion, criteria)
	return execNodeResult, executionResultInfo, execNodeErr
}

func (f *FailoverTransactionProvider) TransactionResultsByBlockID(
	ctx context.Context,
	block *flow.Block,
	encodingVersion entities.EventEncodingVersion,
	criteria optimistic_sync.Criteria,
) ([]*accessmodel.TransactionResult, *optimistic_sync.ExecutionResultInfo, error) {
	localResults, executionResultInfo, localErr := f.localProvider.TransactionResultsByBlockID(ctx, block,
		encodingVersion, criteria)
	if localErr == nil {
		return localResults, executionResultInfo, nil
	}

	execNodeResults, executionResultInfo, execNodeErr := f.execNodeProvider.TransactionResultsByBlockID(ctx, block,
		encodingVersion,
		criteria)
	return execNodeResults, executionResultInfo, execNodeErr
}
