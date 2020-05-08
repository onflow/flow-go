package test

import (
	"bytes"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/crypto/random"
	"github.com/dapperlabs/flow-go/engine/execution/state/delta"
	"github.com/dapperlabs/flow-go/engine/execution/testutil"
	"github.com/dapperlabs/flow-go/engine/verification"
	chmodel "github.com/dapperlabs/flow-go/model/chunks"
	"github.com/dapperlabs/flow-go/model/flow"
	network "github.com/dapperlabs/flow-go/network/mock"
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

		//bootstrap with root account as it is retrieved by VM to check for permissions
		view := delta.NewView(func(key flow.RegisterID) (flow.RegisterValue, error) {
			return nil, nil
		})
		err := testutil.BootstrapLedgerWithRootAccount(view)
		require.NoError(t, err)

		rootRegisterIDs, rootRegisterValues := view.Interactions().Delta.RegisterUpdates()

		ids = append(ids, id1, id2)
		ids = append(ids, rootRegisterIDs...)
		values = append(values, value1, value2)
		values = append(values, rootRegisterValues...)

		unittest.RunWithTempDir(t, func(dir string) {
			f, err := ledger.NewTrieStorage(dir)
			defer f.Done()
			require.NoError(t, err)

			startState, err := f.UpdateRegisters(ids, values, f.EmptyStateCommitment())
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

// SetupMockVerifierEng sets up a mock verifier engine that asserts the followings:
// - that a set of chunks are delivered to it.
// - that each chunk is delivered exactly once
// SetupMockVerifierEng returns the mock engine and a wait group that unblocks when all ERs are received.
func SetupMockVerifierEng(t testing.TB, vChunks []*verification.VerifiableChunk) (*network.Engine, *sync.WaitGroup) {
	eng := new(network.Engine)

	// keep track of which verifiable chunks we have received
	receivedChunks := make(map[flow.Identifier]struct{})
	var (
		// decrement the wait group when each verifiable chunk received
		wg sync.WaitGroup
		// check one verifiable chunk at a time to ensure dupe checking works
		mu sync.Mutex
	)

	// computes expected number of assigned chunks
	expected := 0
	for _, c := range vChunks {
		if IsAssigned(c.ChunkIndex) {
			expected++
		}
	}
	wg.Add(expected)

	eng.On("ProcessLocal", testifymock.Anything).
		Run(func(args testifymock.Arguments) {
			mu.Lock()
			defer mu.Unlock()

			// the received entity should be a verifiable chunk
			vchunk, ok := args[0].(*verification.VerifiableChunk)
			assert.True(t, ok)

			// retrieves the content of received chunk
			chunk, ok := vchunk.Receipt.ExecutionResult.Chunks.ByIndex(vchunk.ChunkIndex)
			require.True(t, ok, "chunk out of range requested")
			vID := chunk.ID()

			// verifies that it has not seen this chunk before
			_, alreadySeen := receivedChunks[vID]
			if alreadySeen {
				t.Logf("received duplicated chunk (id=%s)", vID)
				t.Fail()
				return
			}

			// ensure the received chunk matches one we expect
			for _, vc := range vChunks {
				if chunk.ID() == vID {
					// mark it as seen and decrement the waitgroup
					receivedChunks[vID] = struct{}{}
					// checks end states match as expected
					if !bytes.Equal(vchunk.EndState, vc.EndState) {
						t.Logf("end states are not equal: expected %x got %x", vchunk.EndState, chunk.EndState)
						t.Fail()
					}
					wg.Done()
					return
				}
			}

			// the received chunk doesn't match any expected ERs
			t.Logf("received unexpected ER (id=%s)", vID)
			t.Fail()
		}).
		Return(nil)

	return eng, &wg
}

// IsAssigned is a helper function that returns true for the even indices in [0, chunkNum-1]
func IsAssigned(index uint64) bool {
	return index%2 == 0
}

func VerifiableChunk(chunkIndex uint64, er verification.CompleteExecutionResult) *verification.VerifiableChunk {
	var endState flow.StateCommitment
	// last chunk
	if int(chunkIndex) == len(er.Receipt.ExecutionResult.Chunks)-1 {
		endState = er.Receipt.ExecutionResult.FinalStateCommit
	} else {
		endState = er.Receipt.ExecutionResult.Chunks[chunkIndex+1].StartState
	}

	return &verification.VerifiableChunk{
		ChunkIndex:    chunkIndex,
		EndState:      endState,
		Block:         er.Block,
		Receipt:       er.Receipt,
		Collection:    er.Collections[chunkIndex],
		ChunkDataPack: er.ChunkDataPacks[chunkIndex],
	}
}

type MockAssigner struct {
	me flow.Identifier
}

func NewMockAssigner(id flow.Identifier) *MockAssigner {
	return &MockAssigner{me: id}
}

// Assign assigns all input chunks to the verifier node
func (m *MockAssigner) Assign(ids flow.IdentityList, chunks flow.ChunkList, rng random.Rand) (*chmodel.Assignment, error) {
	if len(chunks) == 0 {
		return nil, fmt.Errorf("assigner called with empty chunk list")
	}
	a := chmodel.NewAssignment()
	for _, c := range chunks {
		if IsAssigned(c.Index) {
			a.Add(c, flow.IdentifierList{m.me})
		}
	}

	return a, nil
}
