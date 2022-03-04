package unittest

import (
	"context"
	"sync"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	blockstore "github.com/ipfs/go-ipfs-blockstore"

	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/stretchr/testify/mock"
)

func TestDatastore() datastore.Batching {
	return dssync.MutexWrap(datastore.NewMapDatastore())
}

func TestBlobService(ds datastore.Batching) *mocknetwork.BlobService {
	bs := blockstore.NewBlockstore(ds)

	bex := new(mocknetwork.BlobService)

	bex.On("GetBlobs", mock.Anything, mock.AnythingOfType("[]cid.Cid")).
		Return(func(ctx context.Context, cids []cid.Cid) <-chan blobs.Blob {
			ch := make(chan blobs.Blob)

			var wg sync.WaitGroup
			wg.Add(len(cids))

			for _, c := range cids {
				c := c
				go func() {
					defer wg.Done()

					blob, err := bs.Get(ctx, c)

					if err != nil {
						// In the real implementation, Bitswap would keep trying to get the blob from
						// the network indefinitely, sending requests to more and more peers until it
						// eventually finds the blob, or the context is canceled. Here, we know that
						// if the blob is not already in the blobstore, then we will never appear, so
						// we just wait for the context to be canceled.
						<-ctx.Done()

						return
					}

					ch <- blob
				}()
			}

			go func() {
				wg.Wait()
				close(ch)
			}()

			return ch
		})

	bex.On("AddBlobs", mock.Anything, mock.AnythingOfType("[]blocks.Block")).Return(bs.PutMany)

	return bex
}
