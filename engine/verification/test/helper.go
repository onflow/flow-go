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
func CompleteExecutionResultFixture(t *testing.T, chunkCount int) verification.CompleteExecutionResult {
	chunks := make([]*flow.Chunk, 0)
	chunkStates := make([]*flow.ChunkState, 0, chunkCount)
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

		db := unittest.TempLevelDB(t)

		f, err := ledger.NewTrieStorage(db)
		require.NoError(t, err)

		startState, err := f.UpdateRegisters(ids, values)
		require.NoError(t, err)
		regTs, err := f.GetRegisterTouches(ids, startState)
		require.NoError(t, err)

		chunk := &flow.Chunk{
			ChunkBody: flow.ChunkBody{
				CollectionIndex: uint(i),
				StartState:      startState,
				EventCollection: unittest.IdentifierFixture(),
			},
			Index: uint64(i),
		}
		chunks = append(chunks, chunk)

		// creates a chunk state
		chunkState := &flow.ChunkState{
			ChunkID:   chunk.ID(),
			Registers: flow.Ledger{},
		}
		chunkStates = append(chunkStates, chunkState)

		// creates a chunk data pack for the chunk
		chunkDataPack := flow.ChunkDataPack{
			ChunkID:         chunk.ID(),
			StartState:      startState,
			RegisterTouches: regTs,
		}
		chunkDataPacks = append(chunkDataPacks, &chunkDataPack)

		// closing historical states of tries
		// also closes level db storage internally
		err1, err2 := f.CloseStorage()
		require.NoError(t, err1)
		require.NoError(t, err2)

	}

	payload := flow.Payload{
		Identities: unittest.IdentityListFixture(32),
		Guarantees: guarantees,
	}
	header := unittest.BlockHeaderFixture()
	header.PayloadHash = payload.Hash()

	block := flow.Block{
		Header:  header,
		Payload: payload,
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
		ChunkStates:    chunkStates,
		ChunkDataPacks: chunkDataPacks,
	}
}
