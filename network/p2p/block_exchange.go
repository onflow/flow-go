package p2p

import (
	"context"

	bitswap "github.com/ipfs/go-bitswap"
	bsnet "github.com/ipfs/go-bitswap/network"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	exchange "github.com/ipfs/go-ipfs-exchange-interface"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/libp2p/go-libp2p-core/routing"

	"github.com/onflow/flow-go/network"
)

var _ network.BlockExchange = (*BlockExchange)(nil)
var _ network.BlockExchangeFetcher = (*BlockExchangeSession)(nil)

type BlockExchange struct {
	bstore    blockstore.Blockstore
	bsNetwork bsnet.BitSwapNetwork
	bs        *bitswap.Bitswap
	cancel    context.CancelFunc
}

func NewBlockExchange(
	parent context.Context,
	host host.Host,
	r routing.ContentRouting,
	prefix string,
	bstore blockstore.Blockstore,
) *BlockExchange {
	ctx, cancel := context.WithCancel(parent)
	bsNetwork := bsnet.NewFromIpfsHost(host, r, bsnet.Prefix(protocol.ID(prefix)))

	return &BlockExchange{
		bstore:    bstore,
		bsNetwork: bsNetwork,
		bs:        bitswap.New(ctx, bsNetwork, bstore).(*bitswap.Bitswap),
		cancel:    cancel,
	}
}

func (e *BlockExchange) GetBlocks(ctx context.Context, cids ...cid.Cid) (<-chan blocks.Block, error) {
	return e.bs.GetBlocks(ctx, cids)
}

func (e *BlockExchange) HasBlock(block blocks.Block) error {
	return e.bs.HasBlock(block)
}

func (e *BlockExchange) GetSession(ctx context.Context) network.BlockExchangeFetcher {
	return NewBlockExchangeSession(ctx, e)
}

func (e *BlockExchange) Close() {
	e.cancel()
}

type BlockExchangeSession struct {
	session exchange.Fetcher
	ex      *BlockExchange
}

func NewBlockExchangeSession(ctx context.Context, ex *BlockExchange) *BlockExchangeSession {
	return &BlockExchangeSession{
		session: ex.bs.NewSession(ctx),
		ex:      ex,
	}
}

func (s *BlockExchangeSession) GetBlocks(ctx context.Context, cids ...cid.Cid) (<-chan blocks.Block, error) {
	return s.session.GetBlocks(ctx, cids)
}
