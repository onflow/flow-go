package unittest

import (
	"context"
	"sync"

	"github.com/ipfs/go-cid"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/module/state_synchronization"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/stretchr/testify/mock"
)

func WithCollections(collections []*flow.Collection) func(*state_synchronization.ExecutionData) {
	return func(executionData *state_synchronization.ExecutionData) {
		executionData.Collections = collections
	}
}

func WithEvents(events []flow.EventsList) func(*state_synchronization.ExecutionData) {
	return func(executionData *state_synchronization.ExecutionData) {
		executionData.Events = events
	}
}

func WithTrieUpdates(updates []*ledger.TrieUpdate) func(*state_synchronization.ExecutionData) {
	return func(executionData *state_synchronization.ExecutionData) {
		executionData.TrieUpdates = updates
	}
}

func ExecutionDataFixture(blockID flow.Identifier) *state_synchronization.ExecutionData {
	return &state_synchronization.ExecutionData{
		BlockID:     blockID,
		Collections: []*flow.Collection{},
		Events:      []flow.EventsList{},
		TrieUpdates: []*ledger.TrieUpdate{},
	}
}

func MockBlobService(bs blockstore.Blockstore) *mocknetwork.BlobService {
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

	noop := module.NoopReadyDoneAware{}
	bex.On("Ready").Return(func() <-chan struct{} { return noop.Ready() })

	return bex
}
