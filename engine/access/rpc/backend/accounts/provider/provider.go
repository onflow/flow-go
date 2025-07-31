package provider

import (
	"context"

	"github.com/onflow/flow-go/model/flow"
)

type AccountProvider interface {
	GetAccountAtBlock(ctx context.Context, address flow.Address, blockID flow.Identifier, height uint64) (*flow.Account, error)
	GetAccountBalanceAtBlock(ctx context.Context, address flow.Address, blockID flow.Identifier, height uint64) (uint64, error)
	GetAccountKeyAtBlock(ctx context.Context, address flow.Address, keyIndex uint32, blockID flow.Identifier, height uint64) (*flow.AccountPublicKey, error)
	GetAccountKeysAtBlock(ctx context.Context, address flow.Address, blockID flow.Identifier, height uint64) ([]flow.AccountPublicKey, error)
}
