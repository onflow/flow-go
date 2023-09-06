package unittest

import (
	"context"
	"fmt"
	"sync"

	"github.com/ipfs/go-cid"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/stretchr/testify/mock"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/network/mocknetwork"
	statemock "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
)

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
		}).Maybe()

	bex.On("AddBlobs", mock.Anything, mock.AnythingOfType("[]blocks.Block")).Return(bs.PutMany).Maybe()
	bex.On("DeleteBlob", mock.Anything, mock.AnythingOfType("cid.Cid")).Return(bs.DeleteBlock).Maybe()

	noop := module.NoopReadyDoneAware{}
	bex.On("Ready").Return(func() <-chan struct{} { return noop.Ready() }).Maybe()

	return bex
}

type SnapshotMockOptions func(*statemock.Snapshot)

func WithHead(head *flow.Header) SnapshotMockOptions {
	return func(snapshot *statemock.Snapshot) {
		snapshot.On("Head").Return(head, nil)
	}
}

func WithSeal(seal *flow.Seal) SnapshotMockOptions {
	return func(snapshot *statemock.Snapshot) {
		snapshot.On("Seal").Return(seal, nil)
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

func WithSealedSnapshot(snapshot *statemock.Snapshot) StateMockOptions {
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
				if _, has := blocksByHeight[height]; !has {
					return nil
				}
				return blocksByHeight[height].Header
			},
			func(height uint64) error {
				if _, has := blocksByHeight[height]; !has {
					return fmt.Errorf("block %d not found: %w", height, storage.ErrNotFound)
				}
				return nil
			},
		)
	}
}

func WithByID(blocksByID map[flow.Identifier]*flow.Block) BlockHeaderMockOptions {
	return func(blocks *storagemock.Headers) {
		blocks.On("ByBlockID", mock.AnythingOfType("flow.Identifier")).Return(
			func(blockID flow.Identifier) *flow.Header {
				if _, has := blocksByID[blockID]; !has {
					return nil
				}
				return blocksByID[blockID].Header
			},
			func(blockID flow.Identifier) error {
				if _, has := blocksByID[blockID]; !has {
					return fmt.Errorf("block %s not found: %w", blockID, storage.ErrNotFound)
				}
				return nil
			},
		)
	}
}

func WithBlockIDByHeight(blocksByHeight map[uint64]*flow.Block) BlockHeaderMockOptions {
	return func(blocks *storagemock.Headers) {
		blocks.On("BlockIDByHeight", mock.AnythingOfType("uint64")).Return(
			func(height uint64) flow.Identifier {
				if _, has := blocksByHeight[height]; !has {
					return flow.ZeroID
				}
				return blocksByHeight[height].Header.ID()
			},
			func(height uint64) error {
				if _, has := blocksByHeight[height]; !has {
					return fmt.Errorf("block %d not found: %w", height, storage.ErrNotFound)
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
				if _, has := resultsByID[blockID]; !has {
					return nil
				}
				return resultsByID[blockID]
			},
			func(blockID flow.Identifier) error {
				if _, has := resultsByID[blockID]; !has {
					return fmt.Errorf("result %s not found: %w", blockID, storage.ErrNotFound)
				}
				return nil
			},
		)
	}
}

func WithResultByID(resultsByID map[flow.Identifier]*flow.ExecutionResult) ResultsMockOptions {
	return func(results *storagemock.ExecutionResults) {
		results.On("ByID", mock.AnythingOfType("flow.Identifier")).Return(
			func(resultID flow.Identifier) *flow.ExecutionResult {
				if _, has := resultsByID[resultID]; !has {
					return nil
				}
				return resultsByID[resultID]
			},
			func(resultID flow.Identifier) error {
				if _, has := resultsByID[resultID]; !has {
					return fmt.Errorf("result %s not found: %w", resultID, storage.ErrNotFound)
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

type SealsMockOptions func(*storagemock.Seals)

func WithSealsByBlockID(sealsByBlockID map[flow.Identifier]*flow.Seal) SealsMockOptions {
	return func(seals *storagemock.Seals) {
		seals.On("FinalizedSealForBlock", mock.AnythingOfType("flow.Identifier")).Return(
			func(blockID flow.Identifier) *flow.Seal {
				if _, has := sealsByBlockID[blockID]; !has {
					return nil
				}
				return sealsByBlockID[blockID]
			},
			func(blockID flow.Identifier) error {
				if _, has := sealsByBlockID[blockID]; !has {
					return fmt.Errorf("seal for block %s not found: %w", blockID, storage.ErrNotFound)
				}
				return nil
			},
		)
	}
}

func MockSealsStorage(opts ...SealsMockOptions) *storagemock.Seals {
	seals := new(storagemock.Seals)

	for _, opt := range opts {
		opt(seals)
	}

	return seals
}

func RemoveExpectedCall(method string, expectedCalls []*mock.Call) []*mock.Call {
	for i, call := range expectedCalls {
		if call.Method == method {
			expectedCalls = append(expectedCalls[:i], expectedCalls[i+1:]...)
		}
	}
	return expectedCalls
}
