package test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine/verification"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/ledger"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// CompleteExecutionResultFixture returns complete execution result with an
// execution receipt referencing the block/collections.
// chunkCount determines the number of chunks inside each receipt
func CompleteExecutionResultFixture(t testing.TB, chunkCount int) verification.CompleteExecutionResult {
	chunks := make([]*flow.Chunk, 0)
	collections := make([]*flow.Collection, 0, chunkCount)
	guarantees := make([]*flow.CollectionGuarantee, 0, chunkCount)
	chunkDataPacks := make([]*flow.ChunkDataPack, 0, chunkCount)

	for i := 0; i < chunkCount; i++ {
		// creates one guaranteed collection per chunk
		coll := unittest.CollectionFixture(3)
		guarantee := coll.Guarantee()
		collections = append(collections, &coll)
		guarantees = append(guarantees, &guarantee)

		// registerTouch and State setup
		id1 := make([]byte, 32)
		value1 := []byte{'a'}

		id2 := make([]byte, 32)
		id2[0] = byte(5)
		value2 := []byte{'b'}

		ids := make([][]byte, 0)
		values := make([][]byte, 0)
		ids = append(ids, id1, id2)
		values = append(values, value1, value2)

		unittest.RunWithTempDir(t, func(dir string) {
			f, err := ledger.NewTrieStorage(dir)
			defer f.Done()
			if t != nil {
				require.NoError(t, err)
			}
			startState, err := f.UpdateRegisters(ids, values, f.EmptyStateCommitment())
			if t != nil {
				require.NoError(t, err)
			}
			regTs, err := f.GetRegisterTouches(ids, startState)
			if t != nil {
				require.NoError(t, err)
			}

			chunk := &flow.Chunk{
				ChunkBody: flow.ChunkBody{
					CollectionIndex: uint(i),
					StartState:      startState,
					EventCollection: unittest.IdentifierFixture(),
				},
				Index: uint64(i),
			}
			chunks = append(chunks, chunk)

			// creates a chunk data pack for the chunk
			chunkDataPack := flow.ChunkDataPack{
				ChunkID:         chunk.ID(),
				StartState:      startState,
				RegisterTouches: regTs,
			}
			chunkDataPacks = append(chunkDataPacks, &chunkDataPack)
		})
	}

	payload := flow.Payload{
		Identities: nil,
		Guarantees: guarantees,
	}
	header := unittest.BlockHeaderFixture()
	header.Height = 0
	header.PayloadHash = payload.Hash()

	block := flow.Block{
		Header:  &header,
		Payload: &payload,
	}

	result := flow.ExecutionResult{
		ExecutionResultBody: flow.ExecutionResultBody{
			BlockID: block.ID(),
			Chunks:  chunks,
		},
	}

	receipt := flow.ExecutionReceipt{
		ExecutionResult: result,
	}

	return verification.CompleteExecutionResult{
		Receipt:        &receipt,
		Block:          &block,
		Collections:    collections,
		ChunkDataPacks: chunkDataPacks,
	}
}
