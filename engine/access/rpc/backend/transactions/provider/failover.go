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
	collectionID flow.Identifier,
	encodingVersion entities.EventEncodingVersion,
) (*accessmodel.TransactionResult, error) {
	localResult, localErr := f.localProvider.TransactionResult(ctx, header, txID, collectionID, encodingVersion)
	if localErr == nil {
		return localResult, nil
	}

	return f.execNodeProvider.TransactionResult(ctx, header, txID, collectionID, encodingVersion)
}

func (f *FailoverTransactionProvider) TransactionResultByIndex(
	ctx context.Context,
	block *flow.Block,
	index uint32,
	collectionID flow.Identifier,
	encodingVersion entities.EventEncodingVersion,
) (*accessmodel.TransactionResult, error) {
	localResult, localErr := f.localProvider.TransactionResultByIndex(ctx, block, index, collectionID, encodingVersion)
	if localErr == nil {
		return localResult, nil
	}

	return f.execNodeProvider.TransactionResultByIndex(ctx, block, index, collectionID, encodingVersion)
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

	return f.execNodeProvider.TransactionResultsByBlockID(ctx, block, encodingVersion)
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
