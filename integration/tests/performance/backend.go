package performance

import (
	"context"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/model/flow"
)

var _ access.API = Backend{}

type Backend struct {

}

func (b Backend) Ping(ctx context.Context) error {
	panic("implement me")
}

func (b Backend) GetNetworkParameters(ctx context.Context) access.NetworkParameters {
	panic("implement me")
}

func (b Backend) GetLatestBlockHeader(ctx context.Context, isSealed bool) (*flow.Header, error) {
	panic("implement me")
}

func (b Backend) GetBlockHeaderByHeight(ctx context.Context, height uint64) (*flow.Header, error) {
	panic("implement me")
}

func (b Backend) GetBlockHeaderByID(ctx context.Context, id flow.Identifier) (*flow.Header, error) {
	panic("implement me")
}

func (b Backend) GetLatestBlock(ctx context.Context, isSealed bool) (*flow.Block, error) {
	panic("implement me")
}

func (b Backend) GetBlockByHeight(ctx context.Context, height uint64) (*flow.Block, error) {
	panic("implement me")
}

func (b Backend) GetBlockByID(ctx context.Context, id flow.Identifier) (*flow.Block, error) {
	panic("implement me")
}

func (b Backend) GetCollectionByID(ctx context.Context, id flow.Identifier) (*flow.LightCollection, error) {
	panic("implement me")
}

func (b Backend) SendTransaction(ctx context.Context, tx *flow.TransactionBody) error {
	panic("implement me")
}

func (b Backend) GetTransaction(ctx context.Context, id flow.Identifier) (*flow.TransactionBody, error) {
	panic("implement me")
}

func (b Backend) GetTransactionResult(ctx context.Context, id flow.Identifier) (*access.TransactionResult, error) {
	panic("implement me")
}

func (b Backend) GetAccount(ctx context.Context, address flow.Address) (*flow.Account, error) {
	panic("implement me")
}

func (b Backend) GetAccountAtLatestBlock(ctx context.Context, address flow.Address) (*flow.Account, error) {
	panic("implement me")
}

func (b Backend) GetAccountAtBlockHeight(ctx context.Context, address flow.Address, height uint64) (*flow.Account, error) {
	panic("implement me")
}

func (b Backend) ExecuteScriptAtLatestBlock(ctx context.Context, script []byte, arguments [][]byte) ([]byte, error) {
	panic("implement me")
}

func (b Backend) ExecuteScriptAtBlockHeight(ctx context.Context, blockHeight uint64, script []byte, arguments [][]byte) ([]byte, error) {
	panic("implement me")
}

func (b Backend) ExecuteScriptAtBlockID(ctx context.Context, blockID flow.Identifier, script []byte, arguments [][]byte) ([]byte, error) {
	panic("implement me")
}

func (b Backend) GetEventsForHeightRange(ctx context.Context, eventType string, startHeight, endHeight uint64) ([]flow.BlockEvents, error) {
	panic("implement me")
}

func (b Backend) GetEventsForBlockIDs(ctx context.Context, eventType string, blockIDs []flow.Identifier) ([]flow.BlockEvents, error) {
	panic("implement me")
}

