package provider

import (
	"context"

	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
)

type AccountProvider interface {
	GetAccountAtBlock(ctx context.Context, address flow.Address, blockID flow.Identifier, height uint64, executionResultInfo *optimistic_sync.ExecutionResultInfo) (*flow.Account, *accessmodel.ExecutorMetadata, error)
	GetAccountBalanceAtBlock(ctx context.Context, address flow.Address, blockID flow.Identifier, height uint64, executionResultInfo *optimistic_sync.ExecutionResultInfo) (uint64, *accessmodel.ExecutorMetadata, error)
	GetAccountKeyAtBlock(ctx context.Context, address flow.Address, keyIndex uint32, blockID flow.Identifier, height uint64, executionResultInfo *optimistic_sync.ExecutionResultInfo) (*flow.AccountPublicKey, *accessmodel.ExecutorMetadata, error)
	GetAccountKeysAtBlock(ctx context.Context, address flow.Address, blockID flow.Identifier, height uint64, executionResultInfo *optimistic_sync.ExecutionResultInfo) ([]flow.AccountPublicKey, *accessmodel.ExecutorMetadata, error)
}
