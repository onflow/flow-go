package test

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/onflow/cadence/runtime"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine/execution/computation/computer"
	"github.com/dapperlabs/flow-go/engine/execution/computation/virtualmachine"
	"github.com/dapperlabs/flow-go/engine/execution/state"
	"github.com/dapperlabs/flow-go/engine/execution/state/bootstrap"
	"github.com/dapperlabs/flow-go/engine/execution/state/delta"
	"github.com/dapperlabs/flow-go/engine/verification"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/mempool/entity"
	"github.com/dapperlabs/flow-go/storage/ledger"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// GetTxBodyDeployCounterContract returns transaction body for deploying counter smart contract
func GetTxBodyDeployCounterContract() *flow.TransactionBody {
	encoded := hex.EncodeToString([]byte(`
			access(all) contract Container {
				access(all) resource Counter {
					pub var count: Int
		
					init(_ v: Int) {
						self.count = v
					}
					pub fun add(_ count: Int) {
						self.count = self.count + count
					}
				}
				pub fun createCounter(_ v: Int): @Counter {
					return <-create Counter(v)
				}
			}`))

	return &flow.TransactionBody{
		Script: []byte(fmt.Sprintf(`transaction {
              prepare(signer: AuthAccount) {
                signer.setCode("%s".decodeHex())
              }
			}`, encoded)),
		ScriptAccounts: []flow.Address{flow.RootAddress},
	}
}

// GetTxBodyCreateCounter returns transaction body for creating a new counter inside the smart contract
func GetTxBodyCreateCounter() *flow.TransactionBody {
	return &flow.TransactionBody{
		Script: []byte(`
			import 0x01
			transaction {
				prepare(acc: AuthAccount) {
					if acc.storage[Container.Counter] == nil {
                		let existing <- acc.storage[Container.Counter] <- Container.createCounter(3)
                		destroy existing
					}
              	}
            }`),
		ScriptAccounts: []flow.Address{flow.RootAddress},
	}
}

// GetTxBodyAddToCounter returns transaction body for adding a value to the counter
func GetTxBodyAddToCounter() *flow.TransactionBody {
	return &flow.TransactionBody{
		Script: []byte(`
			import 0x01
			transaction {
				prepare(acc: AuthAccount) {
					acc.storage[Container.Counter]?.add(2)
              	}
            }`),
		ScriptAccounts: []flow.Address{flow.RootAddress},
	}
}

func GetCompleteExecutionResultForCounter(t *testing.T) verification.CompleteExecutionResult {

	// setup collection
	transactions := make([]*flow.TransactionBody, 0)
	transactions = append(transactions, GetTxBodyDeployCounterContract())
	transactions = append(transactions, GetTxBodyCreateCounter())
	transactions = append(transactions, GetTxBodyAddToCounter())
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
		Header:  header,
		Payload: payload,
	}

	// Setup chunk and chunk data package
	chunks := make([]*flow.Chunk, 0)
	chunkDataPacks := make([]*flow.ChunkDataPack, 0)

	unittest.RunWithTempDBDir(t, func(dir string) {
		led, err := ledger.NewTrieStorage(dir)
		require.NoError(t, err)
		defer led.Done()

		startStateCommitment, err := bootstrap.BootstrapLedger(led)
		require.NoError(t, err)

		rt := runtime.NewInterpreterRuntime()
		vm := virtualmachine.New(rt)

		// create state.View
		view := delta.NewView(state.LedgerGetRegister(led, startStateCommitment))

		// create BlockComputer
		bc := computer.NewBlockComputer(vm)

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
		_, err = bc.ExecuteBlock(executableBlock, view)
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
		allRegisters := view.Interactions().RegisterTouches()
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
		}
		chunkDataPacks = append(chunkDataPacks, chdp)

	})

	result := flow.ExecutionResult{
		ExecutionResultBody: flow.ExecutionResultBody{
			BlockID:          block.ID(),
			Chunks:           chunks,
			FinalStateCommit: chunks[0].EndState,
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
