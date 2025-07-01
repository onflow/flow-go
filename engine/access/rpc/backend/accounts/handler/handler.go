package handler

import (
	"context"

	"github.com/onflow/flow-go/model/flow"
)

//TODO: consider better naming: Resolver (GQL style), Handler (RESTful style), etc.

type Handler interface {
	GetAccount(ctx context.Context, address flow.Address) (*flow.Account, error)
	GetAccountAtLatestBlock(ctx context.Context, address flow.Address) (*flow.Account, error)
	GetAccountAtBlockHeight(ctx context.Context, address flow.Address, blockID flow.Identifier, height uint64) (*flow.Account, error)

	GetAccountBalanceAtLatestBlock(ctx context.Context, address flow.Address) (uint64, error)
	GetAccountBalanceAtBlockHeight(ctx context.Context, address flow.Address, blockID flow.Identifier, height uint64) (uint64, error)

	GetAccountKeyAtLatestBlock(ctx context.Context, address flow.Address, keyIndex uint32) (*flow.AccountPublicKey, error)
	GetAccountKeyAtBlockHeight(ctx context.Context, address flow.Address, keyIndex uint32, blockID flow.Identifier, height uint64) (*flow.AccountPublicKey, error)

	GetAccountKeysAtLatestBlock(ctx context.Context, address flow.Address) ([]flow.AccountPublicKey, error)
	GetAccountKeysAtBlockHeight(ctx context.Context, address flow.Address, blockID flow.Identifier, height uint64) ([]flow.AccountPublicKey, error)
}

//TODO: maybe we'll need some CommonRequester type which can be 'inherited' from to reduce code duplication
