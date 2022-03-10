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
}

var _ network.BlobService = (*blobService)(nil)
var _ component.Component = (*blobService)(nil)

// NewBlobService creates a new BlobService that only interacts with the provided datastore
func NewBlobService(ds datastore.Batching) *blobService {
	bstore := blockstore.NewBlockstore(ds)
	bs := &blobService{
		blockStore: bstore,
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

func (bs *blobService) HasBlob(ctx context.Context, c cid.Cid) (bool, error) {
	return bs.blockStore.Has(ctx, c)
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
