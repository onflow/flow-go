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

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
)

var _ network.EntityExchange = (*EntityExchange)(nil)
var _ network.EntityExchangeFetcher = (*EntityExchangeSession)(nil)

func newEntityBlockStore(_ network.EntityStore) blockstore.Blockstore {
	// TODO
	return nil
}

func entityToBlock(_ flow.Entity) blocks.Block {
	// TODO
}

func blockToEntity(_ flow.Block) flow.Entity {
	// TODO:
	// need to know the entity's hasher function
	//
}

type EntityExchange struct {
	bstore    blockstore.Blockstore
	estore    network.EntityStore
	bsNetwork bsnet.BitSwapNetwork
	bs        *bitswap.Bitswap
}

func NewEntityExchange(
	ctx context.Context,
	host host.Host,
	r routing.ContentRouting,
	prefix string,
	estore network.EntityStore,
) *EntityExchange {
	bstore := newEntityBlockStore(estore)
	bsNetwork := bsnet.NewFromIpfsHost(host, r, bsnet.Prefix(protocol.ID(prefix)))

	return &EntityExchange{
		bstore:    bstore,
		bsNetwork: bsNetwork,
		bs:        bitswap.New(ctx, bsNetwork, bstore).(*bitswap.Bitswap),
	}
}

func (e *EntityExchange) GetEntities(ids ...flow.Identifier) network.EntitiesPromise {
	return &EntitiesPromise{
		blocks: func(ctx context.Context) (<-chan blocks.Block, error) {
			session := e.GetSession(ctx)
			// TODO: convert flow IDs to cids
			var cids []cid.Cid
			return e.session.GetBlocks(ctx, cids)
		},
	}
	session := e.GetSession(ctx)
	return session.GetEntities(ids...)
}

func (e *EntityExchange) HasEntity(entity flow.Entity) {
	// Encode entity using encoder
	e.es
	// TODO: convert entity to block somehow?
	// serialize the entity using interface method,
	// then create hasher based on interface method
	// and create CID that way.
	var blk blocks.Block
	e.bs.HasBlock(blk)
	// TODO: put the entity to datastore?
	// e.bstore.Put()
}

func (e *EntityExchange) GetSession(ctx context.Context) network.EntityExchangeFetcher {
	return NewEntityExchangeSession(e.bs.NewSession(ctx))
}

type EntityExchangeSession struct {
	session exchange.Fetcher
}

func NewEntityExchangeSession(session exchange.Fetcher) *EntityExchangeSession {
	return &EntityExchangeSession{session}
}

func (e *EntityExchangeSession) GetEntities(ids ...flow.Identifier) network.EntitiesPromise {
	return &EntitiesPromise{
		blocks: func(ctx context.Context) (<-chan blocks.Block, error) {
			// TODO: convert flow IDs to cids
			var cids []cid.Cid
			return e.session.GetBlocks(ctx, cids)
		},
	}
}

type EntitiesPromise struct {
	forEach func(flow.Entity)
	blocks  func(context.Context) (<-chan blocks.Block, error)
}

func (e *EntitiesPromise) ForEach(cb func(flow.Entity)) network.EntitiesPromise {
	e.forEach = cb
	return e
}

func (e *EntitiesPromise) Send(ctx context.Context) error {
	cb := e.forEach

	if cb == nil {
		// TODO: return error here since ForEach was not called?
	}

	blocks, err := e.blocks(ctx)
	if err != nil {
		// TODO
	}

	go func() {
		for block := range blocks {
			// convert the block's cid to a flow Identifier
			// id := flow.IDFromCid(block.Cid())
			// TODO: verify if block data matches cid?
			// or is this done for us by bitswap?
			// TODO: convert into entity somehow?
			// we can reverse the fingerprinting process probably?
			var entity flow.Entity
			e.forEach(entity)
		}
	}()

	return nil
}
