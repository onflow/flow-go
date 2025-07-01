package handler

import (
	"context"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
)

type Compare struct {
	log   zerolog.Logger
	state protocol.State
}

var _ Handler = (*Compare)(nil)

func NewCompareHandler(log zerolog.Logger, state protocol.State) *Compare {
	return &Compare{
		log:   log,
		state: state,
	}
}

func (c *Compare) GetAccount(ctx context.Context, address flow.Address) (*flow.Account, error) {
	//TODO implement me
	panic("implement me")
}

func (c *Compare) GetAccountAtLatestBlock(ctx context.Context, address flow.Address) (*flow.Account, error) {
	//TODO implement me
	panic("implement me")
}

func (c *Compare) GetAccountAtBlockHeight(ctx context.Context, address flow.Address, blockID flow.Identifier, height uint64) (*flow.Account, error) {
	//TODO implement me
	panic("implement me")
}

func (c *Compare) GetAccountBalanceAtLatestBlock(ctx context.Context, address flow.Address) (uint64, error) {
	//TODO implement me
	panic("implement me")
}

func (c *Compare) GetAccountBalanceAtBlockHeight(ctx context.Context, address flow.Address, blockID flow.Identifier, height uint64) (uint64, error) {
	//TODO implement me
	panic("implement me")
}

func (c *Compare) GetAccountKeyAtLatestBlock(ctx context.Context, address flow.Address, keyIndex uint32) (*flow.AccountPublicKey, error) {
	//TODO implement me
	panic("implement me")
}

func (c *Compare) GetAccountKeyAtBlockHeight(ctx context.Context, address flow.Address, keyIndex uint32, blockID flow.Identifier, height uint64) (*flow.AccountPublicKey, error) {
	//TODO implement me
	panic("implement me")
}

func (c *Compare) GetAccountKeysAtLatestBlock(ctx context.Context, address flow.Address) ([]flow.AccountPublicKey, error) {
	//TODO implement me
	panic("implement me")
}

func (c *Compare) GetAccountKeysAtBlockHeight(ctx context.Context, address flow.Address, blockID flow.Identifier, height uint64) ([]flow.AccountPublicKey, error) {
	//TODO implement me
	panic("implement me")
}
