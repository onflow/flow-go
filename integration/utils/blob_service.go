package utils

import (
	"context"
	"fmt"

	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"

	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network"
)

type testBlobService struct {
	component.Component
	blockService blockservice.BlockService
	blockStore   blockstore.Blockstore
}

var _ network.BlobService = (*testBlobService)(nil)
var _ component.Component = (*testBlobService)(nil)

// WithHashOnRead sets whether or not the blobstore will rehash the blob data on read
// When set, calls to GetBlob will fail with an error if the hash of the data in storage does not
// match its CID
func WithHashOnRead(enabled bool) network.BlobServiceOption {
	return func(bs network.BlobService) {
		bs.(*testBlobService).blockStore.HashOnRead(enabled)
	}
}

// NewLocalBlobService creates a new BlobService that only interacts with its local datastore
func NewLocalBlobService(
	ds datastore.Batching,
	opts ...network.BlobServiceOption,
) *testBlobService {
	bs := &testBlobService{
		blockStore: blockstore.NewBlockstore(ds),
	}

	for _, opt := range opts {
		opt(bs)
	}

	cm := component.NewComponentManagerBuilder().
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			bs.blockService = blockservice.New(bs.blockStore, nil)

			ready()

			<-ctx.Done()

			if err := bs.blockService.Close(); err != nil {
				ctx.Throw(err)
			}
		}).
		Build()

	bs.Component = cm

	return bs
}

func (bs *testBlobService) GetBlob(ctx context.Context, c cid.Cid) (blobs.Blob, error) {
	return bs.blockService.GetBlock(ctx, c)
}

func (bs *testBlobService) GetBlobs(ctx context.Context, ks []cid.Cid) <-chan blobs.Blob {
	return bs.blockService.GetBlocks(ctx, ks)
}

func (bs *testBlobService) AddBlob(ctx context.Context, b blobs.Blob) error {
	return bs.blockService.AddBlock(ctx, b)
}

func (bs *testBlobService) AddBlobs(ctx context.Context, blobs []blobs.Blob) error {
	return bs.blockService.AddBlocks(ctx, blobs)
}

func (bs *testBlobService) DeleteBlob(ctx context.Context, c cid.Cid) error {
	return bs.blockService.DeleteBlock(ctx, c)
}

func (bs *testBlobService) GetSession(ctx context.Context) network.BlobGetter {
	return nil
}

func (bs *testBlobService) TriggerReprovide(ctx context.Context) error {
	return fmt.Errorf("not implemented")
}
