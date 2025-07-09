package retriever

import (
	"context"

	"github.com/onflow/flow-go/model/flow"
)

type Retriever interface {
	GetAccountAtBlockHeight(ctx context.Context, address flow.Address, blockID flow.Identifier, height uint64) (*flow.Account, error)
	GetAccountBalanceAtBlockHeight(ctx context.Context, address flow.Address, blockID flow.Identifier, height uint64) (uint64, error)
	GetAccountKeyAtBlockHeight(ctx context.Context, address flow.Address, keyIndex uint32, blockID flow.Identifier, height uint64) (*flow.AccountPublicKey, error)
	GetAccountKeysAtBlockHeight(ctx context.Context, address flow.Address, blockID flow.Identifier, height uint64) ([]flow.AccountPublicKey, error)
}

//TODO: maybe we'll need some CommonRequester type which can be 'inherited' from to reduce code duplication

//TODO: considering renaming it to sth else. We already have handler.go. It might be confusing.
