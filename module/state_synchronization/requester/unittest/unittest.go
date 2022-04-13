package unittest

import (
	"context"
	"fmt"
	"sync"

	"github.com/ipfs/go-cid"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/module/state_synchronization"
	"github.com/onflow/flow-go/module/state_synchronization/requester/status"
	"github.com/onflow/flow-go/network/mocknetwork"
	statemock "github.com/onflow/flow-go/state/protocol/mock"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
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

func BlockEntryFixture(height uint64) *status.BlockEntry {
	blockID := unittest.IdentifierFixture()
	return &status.BlockEntry{
		BlockID: blockID,
		Height:  height,
		ExecutionData: &state_synchronization.ExecutionData{
			BlockID: blockID,
		},
	}
}

type SnapshotMockOptions func(*statemock.Snapshot)

func WithHead(head *flow.Header) SnapshotMockOptions {
	return func(snapshot *statemock.Snapshot) {
		snapshot.On("Head").Return(head, nil)
	}
}

func MockProtocolStateSnapshot(opts ...SnapshotMockOptions) *statemock.Snapshot {
	snapshot := new(statemock.Snapshot)

	for _, opt := range opts {
		opt(snapshot)
	}

	return snapshot
}

type StateMockOptions func(*statemock.State)

func WithSnapshot(snapshot *statemock.Snapshot) StateMockOptions {
	return func(state *statemock.State) {
		state.On("Sealed").Return(snapshot)
	}
}

func MockProtocolState(opts ...StateMockOptions) *statemock.State {
	state := new(statemock.State)

	for _, opt := range opts {
		opt(state)
	}

	return state
}

type BlockHeaderMockOptions func(*storagemock.Headers)

func WithByHeight(blocksByHeight map[uint64]*flow.Block) BlockHeaderMockOptions {
	return func(blocks *storagemock.Headers) {
		blocks.On("ByHeight", mock.AnythingOfType("uint64")).Return(
			func(height uint64) *flow.Header {
				return blocksByHeight[height].Header
			},
			func(height uint64) error {
				if _, has := blocksByHeight[height]; !has {
					return fmt.Errorf("block %d not found", height)
				}
				return nil
			},
		)
	}
}

func WithByID(blocksByID map[flow.Identifier]*flow.Block) BlockHeaderMockOptions {
	return func(blocks *storagemock.Headers) {
		blocks.On("ByID", mock.AnythingOfType("flow.Identifier")).Return(
			func(blockID flow.Identifier) *flow.Header {
				return blocksByID[blockID].Header
			},
			func(blockID flow.Identifier) error {
				if _, has := blocksByID[blockID]; !has {
					return fmt.Errorf("block %s not found", blockID)
				}
				return nil
			},
		)
	}
}

func MockBlockHeaderStorage(opts ...BlockHeaderMockOptions) *storagemock.Headers {
	headers := new(storagemock.Headers)

	for _, opt := range opts {
		opt(headers)
	}

	return headers
}

type ResultsMockOptions func(*storagemock.ExecutionResults)

func WithByBlockID(resultsByID map[flow.Identifier]*flow.ExecutionResult) ResultsMockOptions {
	return func(results *storagemock.ExecutionResults) {
		results.On("ByBlockID", mock.AnythingOfType("flow.Identifier")).Return(
			func(blockID flow.Identifier) *flow.ExecutionResult {
				return resultsByID[blockID]
			},
			func(blockID flow.Identifier) error {
				if _, has := resultsByID[blockID]; !has {
					return fmt.Errorf("result %s not found", blockID)
				}
				return nil
			},
		)
	}
}

func MockResultsStorage(opts ...ResultsMockOptions) *storagemock.ExecutionResults {
	results := new(storagemock.ExecutionResults)

	for _, opt := range opts {
		opt(results)
	}

	return results
}
