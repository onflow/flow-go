package network

import (
	"context"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/ipfs/go-bitswap"
	bsnet "github.com/ipfs/go-bitswap/network"
	blockservice "github.com/ipfs/go-blockservice"
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
)

// BlobGetter is the common interface shared between blobservice sessions and
// the blobservice.
type BlobGetter interface {
	// GetBlob gets the requested blob.
	GetBlob(ctx context.Context, c cid.Cid) (blobs.Blob, error)

	// GetBlobs does a batch request for the given cids, returning blobs as
	// they are found, in no particular order.
	//
	// It may not be able to find all requested blobs (or the context may
	// be canceled). In that case, it will close the channel early. It is up
	// to the consumer to detect this situation and keep track which blobs
	// it has received and which it hasn't.
	GetBlobs(ctx context.Context, ks []cid.Cid) <-chan blobs.Blob
}

// BlobService is a hybrid blob datastore. It stores data in a local
// datastore and may retrieve data from a remote Exchange.
// It uses an internal `datastore.Datastore` instance to store values.
type BlobService interface {
	component.Component
	BlobGetter

	// AddBlob puts a given blob to the underlying datastore
	AddBlob(ctx context.Context, b blobs.Blob) error

	// AddBlobs adds a slice of blobs at the same time using batching
	// capabilities of the underlying datastore whenever possible.
	AddBlobs(ctx context.Context, bs []blobs.Blob) error

	// DeleteBlob deletes the given blob from the blobservice.
	DeleteBlob(ctx context.Context, c cid.Cid) error

	// GetSession creates a new session that allows for controlled exchange of wantlists to decrease the bandwidth overhead.
	GetSession(ctx context.Context) BlobGetter

	TriggerReprovide(ctx context.Context) error
}

type blobService struct {
	*component.ComponentManager
	blockService blockservice.BlockService
	reprovider   provider.Reprovider
}

var _ BlobService = (*blobService)(nil)
var _ component.Component = (*blobService)(nil)

type BlobServiceConfig struct {
	ReprovideInterval time.Duration
}

type BlobServiceOption func(*BlobServiceConfig)

func WithReprovideInterval(d time.Duration) BlobServiceOption {
	return func(config *BlobServiceConfig) {
		config.ReprovideInterval = d
	}
}

func NewBlobService(
	host host.Host,
	r routing.ContentRouting,
	prefix string,
	ds datastore.Batching,
	opts ...BlobServiceOption,
) BlobService {
	bs := &blobService{}
	bstore := blockstore.NewBlockstore(ds)
	bsNetwork := bsnet.NewFromIpfsHost(host, r, bsnet.Prefix(protocol.ID(prefix)))
	config := &BlobServiceConfig{
		ReprovideInterval: 12 * time.Hour,
	}

	for _, opt := range opts {
		opt(config)
	}

	bs.ComponentManager = component.NewComponentManagerBuilder().
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			bs.blockService = blockservice.New(bstore, bitswap.New(ctx, bsNetwork, bstore))

			ready()
		}).
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			bs.reprovider = simple.NewReprovider(ctx, config.ReprovideInterval, r, simple.NewBlockstoreProvider(bstore))

			ready()

			bs.reprovider.Run()
		}).
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			ready()

			<-bs.Ready()
			<-ctx.Done()

			var err *multierror.Error

			err = multierror.Append(err, bs.reprovider.Close())
			err = multierror.Append(err, bs.blockService.Close())

			if err.ErrorOrNil() != nil {
				ctx.Throw(err)
			}
		}).
		Build()

	return bs
}

func (bs *blobService) TriggerReprovide(ctx context.Context) error {
	return bs.reprovider.Trigger(ctx)
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

func (bs *blobService) GetSession(ctx context.Context) BlobGetter {
	return &blobServiceSession{blockservice.NewSession(ctx, bs.blockService)}
}

type blobServiceSession struct {
	session *blockservice.Session
}

var _ BlobGetter = (*blobServiceSession)(nil)

func (s *blobServiceSession) GetBlob(ctx context.Context, c cid.Cid) (blobs.Blob, error) {
	return s.session.GetBlock(ctx, c)
}

func (s *blobServiceSession) GetBlobs(ctx context.Context, ks []cid.Cid) <-chan blobs.Blob {
	return s.session.GetBlocks(ctx, ks)
}
