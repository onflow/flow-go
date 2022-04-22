package p2p

import (
	"context"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/ipfs/go-bitswap"
	bsnet "github.com/ipfs/go-bitswap/network"
	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	provider "github.com/ipfs/go-ipfs-provider"
	"github.com/ipfs/go-ipfs-provider/simple"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/libp2p/go-libp2p-core/routing"

	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network"
)

type blobService struct {
	component.Component
	blockService blockservice.BlockService
	blockStore   blockstore.Blockstore
	reprovider   provider.Reprovider
	config       *BlobServiceConfig
}

var _ network.BlobService = (*blobService)(nil)
var _ component.Component = (*blobService)(nil)

type BlobServiceConfig struct {
	ReprovideInterval time.Duration    // the interval at which the DHT provider entries are refreshed
	BitswapOptions    []bitswap.Option // options to pass to the Bitswap service
	BitswapNetwork    bsnet.BitSwapNetwork
	ContentRouting    routing.ContentRouting
}

// WithReprovideInterval sets the interval at which DHT provider entries are refreshed
func WithReprovideInterval(d time.Duration) network.BlobServiceOption {
	return func(bs network.BlobService) {
		bs.(*blobService).config.ReprovideInterval = d
	}
}

// WithBitswap configures the blobstore to use Bitswap as the underlying exchange
func WithBitswap(host host.Host, r routing.ContentRouting, prefix string) network.BlobServiceOption {
	return func(bs network.BlobService) {
		bs.(*blobService).config.BitswapNetwork = bsnet.NewFromIpfsHost(host, r, bsnet.Prefix(protocol.ID(prefix)))
		bs.(*blobService).config.ContentRouting = r
	}
}

// WithBitswapOptions sets additional options for Bitswap exchange
func WithBitswapOptions(opts ...bitswap.Option) network.BlobServiceOption {
	return func(bs network.BlobService) {
		bs.(*blobService).config.BitswapOptions = opts
	}
}

// WithHashOnRead sets whether or not the blobstore will rehash the blob data on read
// When set, calls to GetBlob will fail with an error if the hash of the data in storage does not
// match its CID
func WithHashOnRead(enabled bool) network.BlobServiceOption {
	return func(bs network.BlobService) {
		bs.(*blobService).blockStore.HashOnRead(enabled)
	}
}

// NewBlobService creates a new BlobService.
func NewBlobService(
	ds datastore.Batching,
	opts ...network.BlobServiceOption,
) *blobService {
	bs := &blobService{
		config: &BlobServiceConfig{
			ReprovideInterval: 12 * time.Hour,
		},
		blockStore: blockstore.NewBlockstore(ds),
	}

	for _, opt := range opts {
		opt(bs)
	}

	builder := component.NewComponentManagerBuilder().
		AddWorker(bs.runBlockService).
		AddWorker(bs.shutdownHandler)

	if bs.config.BitswapNetwork != nil {
		builder.AddWorker(bs.runReprovider)
	}

	bs.Component = builder.Build()

	return bs
}

func (bs *blobService) runBlockService(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	if bs.config.BitswapNetwork == nil {
		bs.blockService = blockservice.New(bs.blockStore, nil)
	} else {
		bs.blockService = blockservice.New(
			bs.blockStore,
			bitswap.New(ctx, bs.config.BitswapNetwork, bs.blockStore, bs.config.BitswapOptions...),
		)
	}

	ready()
}

func (bs *blobService) runReprovider(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	bs.reprovider = simple.NewReprovider(ctx,
		bs.config.ReprovideInterval,
		bs.config.ContentRouting,
		simple.NewBlockstoreProvider(bs.blockStore),
	)

	ready()

	bs.reprovider.Run()
}

func (bs *blobService) shutdownHandler(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	<-bs.Ready() // wait for variables to be initialized
	<-ctx.Done()

	var err *multierror.Error

	if bs.reprovider != nil {
		err = multierror.Append(err, bs.reprovider.Close())
	}

	if bs.config.BitswapNetwork != nil {
		err = multierror.Append(err, bs.blockService.Close())
	}

	if err.ErrorOrNil() != nil {
		ctx.Throw(err)
	}
}

func (bs *blobService) TriggerReprovide(ctx context.Context) error {
	if bs.reprovider != nil {
		return bs.reprovider.Trigger(ctx)
	}
	return nil
}

func (bs *blobService) GetBlob(ctx context.Context, c cid.Cid) (blobs.Blob, error) {
	return bs.blockService.GetBlock(ctx, c)
}

func (bs *blobService) GetBlobs(ctx context.Context, ks []cid.Cid) <-chan blobs.Blob {
	return bs.blockService.GetBlocks(ctx, ks)
}

func (bs *blobService) AddBlob(ctx context.Context, b blobs.Blob) error {
	return bs.blockService.AddBlock(ctx, b)
}

func (bs *blobService) AddBlobs(ctx context.Context, blobs []blobs.Blob) error {
	return bs.blockService.AddBlocks(ctx, blobs)
}

func (bs *blobService) DeleteBlob(ctx context.Context, c cid.Cid) error {
	return bs.blockService.DeleteBlock(ctx, c)
}

func (bs *blobService) GetSession(ctx context.Context) network.BlobGetter {
	return &blobServiceSession{blockservice.NewSession(ctx, bs.blockService)}
}

type blobServiceSession struct {
	session *blockservice.Session
}

var _ network.BlobGetter = (*blobServiceSession)(nil)

func (s *blobServiceSession) GetBlob(ctx context.Context, c cid.Cid) (blobs.Blob, error) {
	return s.session.GetBlock(ctx, c)
}

func (s *blobServiceSession) GetBlobs(ctx context.Context, ks []cid.Cid) <-chan blobs.Blob {
	return s.session.GetBlocks(ctx, ks)
}
