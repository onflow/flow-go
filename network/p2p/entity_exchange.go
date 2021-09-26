package p2p

import (
	"context"
	"time"

	bitswap "github.com/ipfs/go-bitswap"
	bsnet "github.com/ipfs/go-bitswap/network"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	exchange "github.com/ipfs/go-ipfs-exchange-interface"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/routing"
	kdht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network"
)

var _ network.EntityExchange = (*EntityExchange)(nil)
var _ network.EntityExchangeFetcher = (*EntityExchangeSession)(nil)
var _ network.EntityExchangeMiddleware = (*EntityExchangeMiddleware)(nil)

type EntityExchange struct {
	router    routing.ContentRouting
	bstore    blockstore.Blockstore
	bsNetwork bsnet.BitSwapNetwork
	bs        *bitswap.Bitswap
}

func NewEntityExchange(h host.Host, router routing.ContentRouting, bstore blockstore.Blockstore) *EntityExchange {
	bsNetwork := bsnet.NewFromIpfsHost(h, router)
	bs := bitswap.New(context.TODO(), bsNetwork, bstore).(*bitswap.Bitswap)

	return &EntityExchange{
		router,
		bstore,
		bsNetwork,
		bs,
	}
}

func (e *EntityExchange) GetEntities(ctx context.Context, cb func(entity flow.Entity), ids ...flow.Identifier) {
	session := e.GetSession(ctx)
	session.GetEntities(ctx, cb, ids...)
}

func (e *EntityExchange) HasEntity(entity flow.Entity) {
	// TODO: convert entity to block somehow?
	var blk blocks.Block
	e.bs.HasBlock(blk)
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

func (e *EntityExchangeSession) GetEntities(ctx context.Context, cb func(entity flow.Entity), ids ...flow.Identifier) {
	var cids []cid.Cid
	// TODO: convert flow IDs to cids
	blocksChan, err := e.session.GetBlocks(ctx, cids)
	if err != nil {
		// TODO
	}
	go func() {
		for block := range blocksChan {
			// convert the block's cid to a flow Identifier
			// id := flow.IDFromCid(block.Cid())
			// TODO: verify if block data matches cid?
			// or is this done for us by bitswap?
			// TODO: convert into entity somehow?
			// we can reverse the fingerprinting process probably?
			var entity flow.Entity
			cb(entity)
		}
	}()
}

type EntityExchangeMiddleware struct {
	network.Middleware
	network.EntityExchange
	bstore blockstore.Blockstore
}

// TODO: entityExchange needs to wait for Middleware to Start (so that libp2phost is created)
// before it can create itself

// I'm thinking the following:

// func NewEntityExchangeMiddleware(/* all of the middleware args */, dht, blockstore, ...middlewareOpts) {
// 	   NewMiddleware(
//     all the middleware args,
//     append(middlewareOpts, WithDHT(dht))
// )
// }
// then, inside Start, we start the middleware and wait for it to be ready, then we create the EntityExchange

func NewEntityExchangeMiddleware(
	log zerolog.Logger,
	libP2PNodeBuilder NodeBuilder,
	flowID flow.Identifier,
	metrics module.NetworkMetrics,
	rootBlockID flow.Identifier,
	unicastMessageTimeout time.Duration,
	connectionGating bool,
	idTranslator IDTranslator,
	dht *kdht.IpfsDHT,
	bstore blockstore.Blockstore,
	opts ...MiddlewareOption,
) *EntityExchangeMiddleware {
	return &EntityExchangeMiddleware{
		Middleware: NewMiddleware(log,
			libP2PNodeBuilder,
			flowID,
			metrics,
			rootBlockID,
			unicastMessageTimeout,
			connectionGating,
			idTranslator,
			append(opts, WithDHT(dht))...),
		bstore: bstore,
	}
}

func (e *EntityExchangeMiddleware) Start(ctx context.Context) {
	go func() {
		// then, inside Start, we start the middleware and wait for it to be ready, then we create the EntityExchange
		// e.Middleware.Start(ctx)
		var mw *Middleware
		e.EntityExchange = NewEntityExchange(mw.libP2PNode.host, mw.dht, e.bstore)
	}()
}
