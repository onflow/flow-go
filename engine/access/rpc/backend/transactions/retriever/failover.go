package retriever

import (
	"context"

	"github.com/onflow/flow/protobuf/go/flow/entities"

	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
)

type FailoverTransactionRetriever struct {
	localProvider    TransactionRetriever
	execNodeProvider TransactionRetriever
}

var _ TransactionRetriever = (*FailoverTransactionRetriever)(nil)

func NewFailoverTransactionRetriever(local TransactionRetriever, execNode TransactionRetriever) *FailoverTransactionRetriever {
	return &FailoverTransactionRetriever{
		localProvider:    local,
		execNodeProvider: execNode,
	}
}

func (f *FailoverTransactionRetriever) TransactionResult(
	ctx context.Context,
	header *flow.Header,
	txID flow.Identifier,
	encodingVersion entities.EventEncodingVersion,
) (*accessmodel.TransactionResult, error) {
	localResult, localErr := f.localProvider.TransactionResult(ctx, header, txID, encodingVersion)
	if localErr == nil {
		return localResult, nil
	}

	ENResult, ENErr := f.execNodeProvider.TransactionResult(ctx, header, txID, encodingVersion)
	return ENResult, ENErr
}

func (f *FailoverTransactionRetriever) TransactionResultByIndex(
	ctx context.Context,
	block *flow.Block,
	index uint32,
	encodingVersion entities.EventEncodingVersion,
) (*accessmodel.TransactionResult, error) {
	localResult, localErr := f.localProvider.TransactionResultByIndex(ctx, block, index, encodingVersion)
	if localErr == nil {
		return localResult, nil
	}

	ENResult, ENErr := f.execNodeProvider.TransactionResultByIndex(ctx, block, index, encodingVersion)
	return ENResult, ENErr
}

func (f *FailoverTransactionRetriever) TransactionResultsByBlockID(
	ctx context.Context,
	block *flow.Block,
	encodingVersion entities.EventEncodingVersion,
) ([]*accessmodel.TransactionResult, error) {
	localResults, localErr := f.localProvider.TransactionResultsByBlockID(ctx, block, encodingVersion)
	if localErr == nil {
		return localResults, nil
	}

	ENResults, ENErr := f.execNodeProvider.TransactionResultsByBlockID(ctx, block, encodingVersion)
	return ENResults, ENErr
}
