package retriever

import (
	"context"

	"github.com/onflow/flow/protobuf/go/flow/entities"

	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
)

type TransactionRetriever interface {
	TransactionResult(
		ctx context.Context,
		header *flow.Header,
		txID flow.Identifier,
		encodingVersion entities.EventEncodingVersion,
	) (*accessmodel.TransactionResult, error)

	TransactionResultByIndex(
		ctx context.Context,
		block *flow.Block,
		index uint32,
		encodingVersion entities.EventEncodingVersion,
	) (*accessmodel.TransactionResult, error)

	TransactionResultsByBlockID(
		ctx context.Context,
		block *flow.Block,
		encodingVersion entities.EventEncodingVersion,
	) ([]*accessmodel.TransactionResult, error)
}
