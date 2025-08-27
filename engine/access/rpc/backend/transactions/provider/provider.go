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
	TransactionResult(ctx context.Context, header *flow.Header, txID flow.Identifier, encodingVersion entities.EventEncodingVersion, query entities.ExecutionStateQuery) (*accessmodel.TransactionResult, *optimistic_sync.ExecutionResultInfo, error)

	TransactionResultByIndex(ctx context.Context, block *flow.Block, index uint32, encodingVersion entities.EventEncodingVersion, query entities.ExecutionStateQuery) (*accessmodel.TransactionResult, *optimistic_sync.ExecutionResultInfo, error)

	TransactionResultsByBlockID(ctx context.Context, block *flow.Block, encodingVersion entities.EventEncodingVersion, query entities.ExecutionStateQuery) ([]*accessmodel.TransactionResult, *optimistic_sync.ExecutionResultInfo, error)
}
