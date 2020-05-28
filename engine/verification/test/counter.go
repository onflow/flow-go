package test

import (
	"context"
	"testing"

	"github.com/onflow/cadence/runtime"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine/execution/computation/computer"
	"github.com/dapperlabs/flow-go/engine/execution/computation/virtualmachine"
	"github.com/dapperlabs/flow-go/engine/execution/state"
	"github.com/dapperlabs/flow-go/engine/execution/state/bootstrap"
	"github.com/dapperlabs/flow-go/engine/execution/state/delta"
	"github.com/dapperlabs/flow-go/engine/execution/testutil"
	"github.com/dapperlabs/flow-go/engine/verification"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/mempool/entity"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/storage/ledger"
	storage "github.com/dapperlabs/flow-go/storage/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func GetCompleteExecutionResultForCounter(t *testing.T, chunkCount int) verification.CompleteExecutionResult {

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
		vm, err := virtualmachine.New(zerolog.Logger{}, rt)
		require.NoError(t, err)

		// create state.View
		view := delta.NewView(state.LedgerGetRegister(led, startStateCommitment))

		// create BlockComputer
		bc := computer.NewBlockComputer(vm, nil, new(storage.Blocks))

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
