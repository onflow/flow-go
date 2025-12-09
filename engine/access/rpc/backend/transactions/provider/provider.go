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
	TransactionResult(
		ctx context.Context,
		header *flow.Header,
		txID flow.Identifier,
		encodingVersion entities.EventEncodingVersion,
		criteria optimistic_sync.Criteria,
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

	TransactionsByBlockID(
		ctx context.Context,
		block *flow.Block,
	) ([]*flow.TransactionBody, error)

	SystemTransaction(
		ctx context.Context,
		block *flow.Block,
		txID flow.Identifier,
	) (*flow.TransactionBody, error)

	SystemTransactionResult(
		ctx context.Context,
		block *flow.Block,
		txID flow.Identifier,
		requiredEventEncodingVersion entities.EventEncodingVersion,
	) (*accessmodel.TransactionResult, error)
}
