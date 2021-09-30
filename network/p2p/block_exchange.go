package p2p

import (
	"context"
	"errors"
	"fmt"

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
var _ network.BlocksPromise = (*BlocksPromise)(nil)
var _ network.BlocksRequest = (*BlocksRequest)(nil)

type BlockExchange struct {
	bstore    blockstore.Blockstore
	bsNetwork bsnet.BitSwapNetwork
	bs        *bitswap.Bitswap
}

func NewBlockExchange(
	ctx context.Context,
	host host.Host,
	r routing.ContentRouting,
	prefix string,
	bstore blockstore.Blockstore,
) *BlockExchange {
	bsNetwork := bsnet.NewFromIpfsHost(host, r, bsnet.Prefix(protocol.ID(prefix)))

	return &BlockExchange{
		bstore:    bstore,
		bsNetwork: bsNetwork,
		bs:        bitswap.New(ctx, bsNetwork, bstore).(*bitswap.Bitswap),
	}
}

func (e *BlockExchange) GetBlocks(cids ...cid.Cid) network.BlocksPromise {
	return &BlocksPromise{
		blocks: func(ctx context.Context) (<-chan blocks.Block, error) {
			return e.bs.GetBlocks(ctx, cids)
		},
	}
}

func (e *BlockExchange) HasBlock(block blocks.Block) error {
	return e.bs.HasBlock(block)
}

func (e *BlockExchange) GetSession(ctx context.Context) network.BlockExchangeFetcher {
	return NewBlockExchangeSession(ctx, e)
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

func (s *BlockExchangeSession) GetBlocks(cids ...cid.Cid) network.BlocksPromise {
	return &BlocksPromise{
		blocks: func(ctx context.Context) (<-chan blocks.Block, error) {
			return s.session.GetBlocks(ctx, cids)
		},
	}
}

type BlocksPromise struct {
	blocks func(context.Context) (<-chan blocks.Block, error)
}

func (p *BlocksPromise) ForEach(cb func(blocks.Block)) network.BlocksRequest {
	return &BlocksRequest{
		forEach: cb,
		blocks:  p.blocks,
	}
}

type BlocksRequest struct {
	forEach func(blocks.Block)
	blocks  func(context.Context) (<-chan blocks.Block, error)
}

func (p *BlocksRequest) Send(ctx context.Context) error {
	cb := p.forEach

	if cb == nil {
		return errors.New("handler must be set by calling ForEach before request can be sent")
	}

	blocks, err := p.blocks(ctx)
	if err != nil {
		return fmt.Errorf("failed to get channel for blocks: %w", err)
	}

	go func() {
		for block := range blocks {
			p.forEach(block)
		}
	}()

	return nil
}
