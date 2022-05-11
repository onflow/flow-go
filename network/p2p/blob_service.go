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
	reprovider   provider.Reprovider
	config       *BlobServiceConfig
}

var _ network.BlobService = (*blobService)(nil)
var _ component.Component = (*blobService)(nil)

type BlobServiceConfig struct {
	ReprovideInterval time.Duration    // the interval at which the DHT provider entries are refreshed
	BitswapOptions    []bitswap.Option // options to pass to the Bitswap service
}

// WithReprovideInterval sets the interval at which DHT provider entries are refreshed
func WithReprovideInterval(d time.Duration) network.BlobServiceOption {
	return func(bs network.BlobService) {
		bs.(*blobService).config.ReprovideInterval = d
	}
}

func WithBitswapOptions(opts ...bitswap.Option) network.BlobServiceOption {
	return func(bs network.BlobService) {
		bs.(*blobService).config.BitswapOptions = opts
	}
}

// NewBlobService creates a new BlobService.
func NewBlobService(
	host host.Host,
	r routing.ContentRouting,
	prefix string,
	ds datastore.Batching,
	opts ...network.BlobServiceOption,
) *blobService {
	bstore := blockstore.NewBlockstore(ds)
	bsNetwork := bsnet.NewFromIpfsHost(host, r, bsnet.Prefix(protocol.ID(prefix)))
	config := &BlobServiceConfig{
		ReprovideInterval: 12 * time.Hour,
	}
	bs := &blobService{config: config}

	for _, opt := range opts {
		opt(bs)
	}

	cm := component.NewComponentManagerBuilder().
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			bs.blockService = blockservice.New(bstore, bitswap.New(ctx, bsNetwork, bstore, config.BitswapOptions...))

			ready()
		}).
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			bs.reprovider = simple.NewReprovider(ctx, config.ReprovideInterval, r, simple.NewBlockstoreProvider(bstore))

			ready()

			bs.reprovider.Run()
		}).
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			ready()

			<-bs.Ready() // wait for variables to be initialized
			<-ctx.Done()

			var err *multierror.Error

			err = multierror.Append(err, bs.reprovider.Close())
			err = multierror.Append(err, bs.blockService.Close())

			if err.ErrorOrNil() != nil {
				ctx.Throw(err)
			}
		}).
		Build()

	bs.Component = cm

	return bs
}

func (bs *blobService) TriggerReprovide(ctx context.Context) error {
	return bs.reprovider.Trigger(ctx)
}

func (bs *blobService) GetBlob(ctx context.Context, c cid.Cid) (blobs.Blob, error) {
	blob, err := bs.blockService.GetBlock(ctx, c)
	if err == blockservice.ErrNotFound {
		return nil, network.ErrBlobNotFound
	}

	return blob, err
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
