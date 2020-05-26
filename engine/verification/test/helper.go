package test

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/onflow/cadence/runtime"
	"github.com/stretchr/testify/assert"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/crypto/random"
	"github.com/dapperlabs/flow-go/engine/execution/computation/computer"
	"github.com/dapperlabs/flow-go/engine/execution/computation/virtualmachine"
	"github.com/dapperlabs/flow-go/engine/execution/state"
	"github.com/dapperlabs/flow-go/engine/execution/state/bootstrap"
	"github.com/dapperlabs/flow-go/engine/execution/state/delta"
	"github.com/dapperlabs/flow-go/engine/execution/testutil"
	"github.com/dapperlabs/flow-go/engine/verification"
	chmodel "github.com/dapperlabs/flow-go/model/chunks"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/mempool/entity"
	"github.com/dapperlabs/flow-go/module/metrics"
	network "github.com/dapperlabs/flow-go/network/mock"
	"github.com/dapperlabs/flow-go/storage/ledger"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// // CompleteExecutionResultFixture returns complete execution result with an
// // execution receipt referencing the block/collections.
// // chunkCount determines the number of chunks inside each receipt
func CompleteExecutionResultFixture(t *testing.T, chunkCount int) verification.CompleteExecutionResult {

	// setup collection
	tx1 := testutil.DeployCounterContractTransaction(flow.ServiceAddress())
	err := testutil.SignTransactionByRoot(tx1, 0)
	require.NoError(t, err)
	tx2 := testutil.CreateCounterTransaction(flow.ServiceAddress(), flow.ServiceAddress())
	err = testutil.SignTransactionByRoot(tx2, 1)
	require.NoError(t, err)
	tx3 := testutil.CreateCounterPanicTransaction(flow.ServiceAddress(), flow.ServiceAddress())
	err = testutil.SignTransactionByRoot(tx3, 2)
	require.NoError(t, err)
	transactions := []*flow.TransactionBody{tx1, tx2, tx3}

	col := flow.Collection{Transactions: transactions}
	collections := []*flow.Collection{&col}

	// setup block
	guarantee := col.Guarantee()
	guarantees := []*flow.CollectionGuarantee{&guarantee}

	payload := flow.Payload{
		Identities: unittest.IdentityListFixture(32),
		Guarantees: guarantees,
	}
	header := unittest.BlockHeaderFixture()
	header.Height = 0
	header.PayloadHash = payload.Hash()

	block := flow.Block{
		Header:  &header,
		Payload: &payload,
	}

	// Setup chunk and chunk data package
	chunks := make([]*flow.Chunk, 0)
	chunkDataPacks := make([]*flow.ChunkDataPack, 0)

	metricsCollector := &metrics.NoopCollector{}

	unittest.RunWithTempDir(t, func(dir string) {
		led, err := ledger.NewMTrieStorage(dir, 100, metricsCollector, nil)
		require.NoError(t, err)
		defer led.Done()

		startStateCommitment, err := bootstrap.BootstrapLedger(
			led,
			unittest.ServiceAccountPublicKey,
			unittest.GenesisTokenSupply,
		)
		require.NoError(t, err)

		rt := runtime.NewInterpreterRuntime()
		vm, err := virtualmachine.New(rt)
		require.NoError(t, err)

		// create state.View
		view := delta.NewView(state.LedgerGetRegister(led, startStateCommitment))

		// create BlockComputer
		bc := computer.NewBlockComputer(vm, nil)

		completeColls := make(map[flow.Identifier]*entity.CompleteCollection)
		completeColls[guarantee.ID()] = &entity.CompleteCollection{
			Guarantee:    &guarantee,
			Transactions: transactions,
		}

		executableBlock := &entity.ExecutableBlock{
			Block:               &block,
			CompleteCollections: completeColls,
			StartState:          startStateCommitment,
		}

		// *execution.ComputationResult, error
		_, err = bc.ExecuteBlock(context.Background(), executableBlock, view)
		require.NoError(t, err, "error executing block")

		ids, values := view.Delta().RegisterUpdates()

		// TODO: update CommitDelta to also return proofs
		endStateCommitment, err := led.UpdateRegisters(ids, values, startStateCommitment)
		require.NoError(t, err, "error updating registers")

		chunk := &flow.Chunk{
			ChunkBody: flow.ChunkBody{
				CollectionIndex: uint(0),
				StartState:      startStateCommitment,
				// TODO: include event collection hash
				EventCollection: flow.ZeroID,
				// TODO: record gas used
				TotalComputationUsed: 0,
				// TODO: record number of txs
				NumberOfTransactions: 0,
			},
			Index:    0,
			EndState: endStateCommitment,
		}
		chunks = append(chunks, chunk)

		// chunkDataPack
		allRegisters := view.Interactions().AllRegisters()
		values, proofs, err := led.GetRegistersWithProof(allRegisters, chunk.StartState)
		require.NoError(t, err, "error reading registers with proofs from ledger")

		regTs := make([]flow.RegisterTouch, len(allRegisters))
		for i, reg := range allRegisters {
			regTs[i] = flow.RegisterTouch{RegisterID: reg,
				Value: values[i],
				Proof: proofs[i],
			}
		}
		chdp := &flow.ChunkDataPack{
			ChunkID:         chunk.ID(),
			StartState:      chunk.StartState,
			RegisterTouches: regTs,
			CollectionID:    col.ID(),
		}
		chunkDataPacks = append(chunkDataPacks, chdp)
		startStateCommitment = endStateCommitment

		for i := 1; i < chunkCount; i++ {

			tx3 = testutil.CreateCounterPanicTransaction(flow.ServiceAddress(), flow.ServiceAddress())
			err = testutil.SignTransactionByRoot(tx3, 3+uint64(i))
			require.NoError(t, err)

			transactions := []*flow.TransactionBody{tx3}
			col := flow.Collection{Transactions: transactions}
			collections = append(collections, &col)
			g := col.Guarantee()
			guarantees = append(guarantees, &g)

			completeColls := make(map[flow.Identifier]*entity.CompleteCollection)
			completeColls[guarantee.ID()] = &entity.CompleteCollection{
				Guarantee:    &guarantee,
				Transactions: transactions,
			}

			executableBlock := &entity.ExecutableBlock{
				Block:               &block,
				CompleteCollections: completeColls,
				StartState:          startStateCommitment,
			}

			// *execution.ComputationResult, error
			_, err = bc.ExecuteBlock(context.Background(), executableBlock, view)
			require.NoError(t, err, "error executing block")

			ids, values := view.Delta().RegisterUpdates()

			// TODO: update CommitDelta to also return proofs
			endStateCommitment, err := led.UpdateRegisters(ids, values, startStateCommitment)
			require.NoError(t, err, "error updating registers")

			chunk := &flow.Chunk{
				ChunkBody: flow.ChunkBody{
					CollectionIndex: uint(i),
					StartState:      startStateCommitment,
					// TODO: include event collection hash
					EventCollection: flow.ZeroID,
					// TODO: record gas used
					TotalComputationUsed: 0,
					// TODO: record number of txs
					NumberOfTransactions: 0,
				},
				Index:    uint64(i),
				EndState: endStateCommitment,
			}
			chunks = append(chunks, chunk)

			// chunkDataPack
			allRegisters := view.Interactions().AllRegisters()
			values, proofs, err := led.GetRegistersWithProof(allRegisters, chunk.StartState)
			require.NoError(t, err, "error reading registers with proofs from ledger")

			regTs := make([]flow.RegisterTouch, len(allRegisters))
			for i, reg := range allRegisters {
				regTs[i] = flow.RegisterTouch{RegisterID: reg,
					Value: values[i],
					Proof: proofs[i],
				}
			}
			chdp := &flow.ChunkDataPack{
				ChunkID:         chunk.ID(),
				StartState:      chunk.StartState,
				RegisterTouches: regTs,
				CollectionID:    col.ID(),
			}
			chunkDataPacks = append(chunkDataPacks, chdp)
			startStateCommitment = endStateCommitment
		}
	})

	payload = flow.Payload{
		Identities: unittest.IdentityListFixture(32),
		Guarantees: guarantees,
	}
	header = unittest.BlockHeaderFixture()
	header.Height = 0
	header.PayloadHash = payload.Hash()

	block = flow.Block{
		Header:  &header,
		Payload: &payload,
	}

	result := flow.ExecutionResult{
		ExecutionResultBody: flow.ExecutionResultBody{
			BlockID:          block.ID(),
			Chunks:           chunks,
			FinalStateCommit: chunks[len(chunks)-1].EndState,
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

// LightExecutionResultFixture returns a light mocked version of execution result with an
// execution receipt referencing the block/collections. In the light version of execution result,
// everything is wired properly, but with the minimum viable content provided. This version is basically used
// for profiling.
func LightExecutionResultFixture(chunkCount int) verification.CompleteExecutionResult {
	chunks := make([]*flow.Chunk, 0)
	collections := make([]*flow.Collection, 0, chunkCount)
	guarantees := make([]*flow.CollectionGuarantee, 0, chunkCount)
	chunkDataPacks := make([]*flow.ChunkDataPack, 0, chunkCount)

	for i := 0; i < chunkCount; i++ {
		// creates one guaranteed collection per chunk
		coll := unittest.CollectionFixture(1)
		guarantee := coll.Guarantee()
		collections = append(collections, &coll)
		guarantees = append(guarantees, &guarantee)

		chunk := &flow.Chunk{
			ChunkBody: flow.ChunkBody{
				CollectionIndex: uint(i),
				EventCollection: unittest.IdentifierFixture(),
			},
			Index: uint64(i),
		}
		chunks = append(chunks, chunk)

		// creates a chunk data pack for the chunk
		chunkDataPack := flow.ChunkDataPack{
			ChunkID: chunk.ID(),
		}
		chunkDataPacks = append(chunkDataPacks, &chunkDataPack)
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
