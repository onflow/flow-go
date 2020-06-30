package utils

import (
	"context"
	"testing"

	"github.com/onflow/cadence/runtime"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine/execution/computation/computer"
	"github.com/dapperlabs/flow-go/engine/execution/state"
	"github.com/dapperlabs/flow-go/engine/execution/state/bootstrap"
	"github.com/dapperlabs/flow-go/engine/execution/state/delta"
	"github.com/dapperlabs/flow-go/engine/execution/testutil"
	"github.com/dapperlabs/flow-go/fvm"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/mempool/entity"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/storage/ledger"
	storage "github.com/dapperlabs/flow-go/storage/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// CompleteExecutionResult represents an execution result that is ready to
// be verified. It contains all execution result and all resources required to
// verify it.
// TODO update this as needed based on execution requirements
type CompleteExecutionResult struct {
	Receipt        *flow.ExecutionReceipt
	Block          *flow.Block
	Collections    []*flow.Collection
	ChunkDataPacks []*flow.ChunkDataPack
}

// CompleteExecutionResultFixture returns complete execution result with an
// execution receipt referencing the block/collections.
// chunkCount determines the number of chunks inside each receipt
func CompleteExecutionResultFixture(t *testing.T, chunkCount int, chain flow.Chain) CompleteExecutionResult {

	// setup collection
	tx1 := testutil.DeployCounterContractTransaction(chain.ServiceAddress(), chain)
	err := testutil.SignTransactionAsServiceAccount(tx1, 0, chain)
	require.NoError(t, err)
	tx2 := testutil.CreateCounterTransaction(chain.ServiceAddress(), chain.ServiceAddress())
	err = testutil.SignTransactionAsServiceAccount(tx2, 1, chain)
	require.NoError(t, err)
	tx3 := testutil.CreateCounterPanicTransaction(chain.ServiceAddress(), chain.ServiceAddress())
	err = testutil.SignTransactionAsServiceAccount(tx3, 2, chain)
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

	log := zerolog.Nop()

	unittest.RunWithTempDir(t, func(dir string) {
		led, err := ledger.NewMTrieStorage(dir, 100, metricsCollector, nil)
		require.NoError(t, err)
		defer led.Done()

		startStateCommitment, err := bootstrap.NewBootstrapper(log).BootstrapLedger(
			led,
			unittest.ServiceAccountPublicKey,
			unittest.GenesisTokenSupply,
			chain,
		)
		require.NoError(t, err)

		rt := runtime.NewInterpreterRuntime()

		vm := fvm.New(rt, chain)

		blocks := new(storage.Blocks)

		execCtx := fvm.NewContext(fvm.WithBlocks(blocks))

		// create state.View
		view := delta.NewView(state.LedgerGetRegister(led, startStateCommitment))

		// create BlockComputer
		bc := computer.NewBlockComputer(vm, execCtx, nil, nil, log)

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

			tx3 = testutil.CreateCounterPanicTransaction(chain.ServiceAddress(), chain.ServiceAddress())
			err = testutil.SignTransactionAsServiceAccount(tx3, 3+uint64(i), chain)
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
	return CompleteExecutionResult{
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
func LightExecutionResultFixture(chunkCount int) CompleteExecutionResult {
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

	return CompleteExecutionResult{
		Receipt:        &receipt,
		Block:          &block,
		Collections:    collections,
		ChunkDataPacks: chunkDataPacks,
	}
}
