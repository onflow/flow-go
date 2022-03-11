package local

import (
	"context"

	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"

	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network"
)

type blobService struct {
	component.Component
	blockService blockservice.BlockService
	blockStore   blockstore.Blockstore
	config       BlobServiceConfig
}

var _ network.BlobService = (*blobService)(nil)
var _ component.Component = (*blobService)(nil)

type BlobServiceConfig struct {
	HashOnRead bool
}

// WithReprovideInterval sets the interval at which DHT provider entries are refreshed
func WithHashOnRead(enabled bool) network.BlobServiceOption {
	return func(bs network.BlobService) {
		bs.(*blobService).blockStore.HashOnRead(enabled)
	}
}

// NewBlobService creates a new BlobService that only interacts with the provided datastore
func NewBlobService(ds datastore.Batching, opts ...network.BlobServiceOption) *blobService {
	bstore := blockstore.NewBlockstore(ds)
	bs := &blobService{
		blockStore: bstore,
	}

	for _, opt := range opts {
		opt(bs)
	}

	cm := component.NewComponentManagerBuilder().
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			bs.blockService = blockservice.New(bstore, nil)

			ready()
		}).
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			ready()

			<-bs.Ready() // wait for variables to be initialized
			<-ctx.Done()

			if err := bs.blockService.Close(); err != nil {
				ctx.Throw(err)
			}
		}).
		Build()

	bs.Component = cm

	return bs
}

func (bs *blobService) TriggerReprovide(ctx context.Context) error {
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
