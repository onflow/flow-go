package provider

import (
	"context"

	"github.com/onflow/flow/protobuf/go/flow/entities"

	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
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
) (*accessmodel.TransactionResult, error) {
	localResult, localErr := f.localProvider.TransactionResult(ctx, header, txID, encodingVersion)
	if localErr == nil {
		return localResult, nil
	}

	execNodeResult, execNodeErr := f.execNodeProvider.TransactionResult(ctx, header, txID, encodingVersion)
	return execNodeResult, execNodeErr
}

func (f *FailoverTransactionProvider) TransactionResultByIndex(
	ctx context.Context,
	block *flow.Block,
	index uint32,
	encodingVersion entities.EventEncodingVersion,
) (*accessmodel.TransactionResult, error) {
	localResult, localErr := f.localProvider.TransactionResultByIndex(ctx, block, index, encodingVersion)
	if localErr == nil {
		return localResult, nil
	}

	execNodeResult, execNodeErr := f.execNodeProvider.TransactionResultByIndex(ctx, block, index, encodingVersion)
	return execNodeResult, execNodeErr
}

func (f *FailoverTransactionProvider) TransactionResultsByBlockID(
	ctx context.Context,
	block *flow.Block,
	encodingVersion entities.EventEncodingVersion,
) ([]*accessmodel.TransactionResult, error) {
	localResults, localErr := f.localProvider.TransactionResultsByBlockID(ctx, block, encodingVersion)
	if localErr == nil {
		return localResults, nil
	}

	execNodeResults, execNodeErr := f.execNodeProvider.TransactionResultsByBlockID(ctx, block, encodingVersion)
	return execNodeResults, execNodeErr
}
