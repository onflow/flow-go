package computation

import (
	"context"
	"fmt"
	"testing"

	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/computation/committer"
	"github.com/onflow/flow-go/engine/execution/computation/computer"
	"github.com/onflow/flow-go/engine/execution/state"
	bootstrapexec "github.com/onflow/flow-go/engine/execution/state/bootstrap"
	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/engine/execution/testutil"
	"github.com/onflow/flow-go/engine/verification/fetcher"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/fvm/programs"
	completeLedger "github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/wal/fixtures"
	chmodels "github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/verification"
	"github.com/onflow/flow-go/module/chunks"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/utils/unittest"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var chain = flow.Mainnet.Chain()

func Test_ExecutionMatchesVerification(t *testing.T) {

	t.Run("empty block", func(t *testing.T) {
		executeBlockAndVerify(t, [][]*flow.TransactionBody{})
	})

	t.Run("single transaction event", func(t *testing.T) {

		deployTx := blueprints.DeployContractTransaction(chain.ServiceAddress(), []byte(""+
			`pub contract Foo {
				pub event FooEvent(x: Int, y: Int)

				pub fun event() { 
					emit FooEvent(x: 2, y: 1)
				}
			}`), "Foo")

		emitTx := &flow.TransactionBody{
			Script: []byte(fmt.Sprintf(`
			import Foo from 0x%s
			transaction {
				prepare() {}
				execute {
					Foo.event()
				}
			}`, chain.ServiceAddress())),
		}

		err := testutil.SignTransactionAsServiceAccount(deployTx, 0, chain)
		require.NoError(t, err)

		err = testutil.SignTransactionAsServiceAccount(emitTx, 1, chain)
		require.NoError(t, err)

		cr := executeBlockAndVerify(t, [][]*flow.TransactionBody{
			{
				deployTx, emitTx,
			},
		})

		// ensure event is emitted
		require.Empty(t, cr.TransactionResults[0].ErrorMessage)
		require.Empty(t, cr.TransactionResults[1].ErrorMessage)
		require.Len(t, cr.Events[0], 2)
		require.Equal(t, flow.EventType(fmt.Sprintf("A.%s.Foo.FooEvent", chain.ServiceAddress())), cr.Events[0][1].Type)
	})

	t.Run("multiple collections events", func(t *testing.T) {

		deployTx := blueprints.DeployContractTransaction(chain.ServiceAddress(), []byte(""+
			`pub contract Foo {
				pub event FooEvent(x: Int, y: Int)

				pub fun event() { 
					emit FooEvent(x: 2, y: 1)
				}
			}`), "Foo")

		emitTx1 := flow.TransactionBody{
			Script: []byte(fmt.Sprintf(`
			import Foo from 0x%s
			transaction {
				prepare() {}
				execute {
					Foo.event()
				}
			}`, chain.ServiceAddress())),
		}

		// copy txs
		emitTx2 := emitTx1
		emitTx3 := emitTx1

		err := testutil.SignTransactionAsServiceAccount(deployTx, 0, chain)
		require.NoError(t, err)

		err = testutil.SignTransactionAsServiceAccount(&emitTx1, 1, chain)
		require.NoError(t, err)
		err = testutil.SignTransactionAsServiceAccount(&emitTx2, 2, chain)
		require.NoError(t, err)
		err = testutil.SignTransactionAsServiceAccount(&emitTx3, 3, chain)
		require.NoError(t, err)

		cr := executeBlockAndVerify(t, [][]*flow.TransactionBody{
			{
				deployTx, &emitTx1,
			},
			{
				&emitTx2,
			},
			{
				&emitTx3,
			},
		})

		// ensure event is emitted
		require.Empty(t, cr.TransactionResults[0].ErrorMessage)
		require.Empty(t, cr.TransactionResults[1].ErrorMessage)
		require.Empty(t, cr.TransactionResults[2].ErrorMessage)
		require.Empty(t, cr.TransactionResults[3].ErrorMessage)
		require.Len(t, cr.Events[0], 2)
		require.Equal(t, flow.EventType(fmt.Sprintf("A.%s.Foo.FooEvent", chain.ServiceAddress())), cr.Events[0][1].Type)
	})
}

func executeBlockAndVerify(t *testing.T, txs [][]*flow.TransactionBody) *execution.ComputationResult {
	rt := fvm.NewInterpreterRuntime()
	vm := fvm.NewVirtualMachine(rt)

	logger := zerolog.Nop()

	fvmContext := fvm.NewContext(logger,
		fvm.WithChain(chain),
		//fvm.WithBlocks(blockFinder),
		fvm.WithTransactionFeesEnabled(true),
		fvm.WithAccountStorageLimit(true),
	)

	collector := metrics.NewNoopCollector()
	tracer := trace.NewNoopTracer()

	wal := &fixtures.NoopWAL{}

	ledger, err := completeLedger.NewLedger(wal, 100, collector, logger, completeLedger.DefaultPathFinderVersion)
	require.NoError(t, err)

	bootstrapper := bootstrapexec.NewBootstrapper(logger)

	initialCommit, err := bootstrapper.BootstrapLedger(
		ledger,
		unittest.ServiceAccountPublicKey,
		chain,
		fvm.WithInitialTokenSupply(unittest.GenesisTokenSupply),
		fvm.WithAccountCreationFee(fvm.DefaultAccountCreationFee),
		fvm.WithMinimumStorageReservation(fvm.DefaultMinimumStorageReservation),
		fvm.WithStorageMBPerFLOW(fvm.DefaultStorageMBPerFLOW),
	)

	require.NoError(t, err)

	ledgerCommiter := committer.NewLedgerViewCommitter(ledger, tracer)

	blockComputer, err := computer.NewBlockComputer(vm, fvmContext, collector, tracer, logger, ledgerCommiter)
	require.NoError(t, err)

	view := delta.NewView(state.LedgerGetRegister(ledger, initialCommit))

	executableBlock := unittest.ExecutableBlockFromTransactions(txs)
	executableBlock.StartState = &initialCommit

	computationResult, err := blockComputer.ExecuteBlock(context.Background(), executableBlock, view, programs.NewEmptyPrograms())
	require.NoError(t, err)

	prevResultId := unittest.IdentifierFixture()

	_, chdps, er, err := execution.GenerateExecutionResultAndChunkDataPacks(prevResultId, initialCommit, computationResult)
	require.NoError(t, err)

	verifier := chunks.NewChunkVerifier(vm, fvmContext, logger)

	vcds := make([]*verification.VerifiableChunkData, er.Chunks.Len())

	for i, chunk := range er.Chunks {
		isSystemChunk := i == er.Chunks.Len()-1
		var collection flow.Collection
		if !isSystemChunk {
			collection = executableBlock.CompleteCollections[chdps[i].CollectionID].Collection()
		}
		offsetForChunk, err := fetcher.TransactionOffsetForChunk(er.Chunks, chunk.Index)
		require.NoError(t, err)

		vcds[i] = &verification.VerifiableChunkData{
			IsSystemChunk:     isSystemChunk,
			Chunk:             chunk,
			Header:            executableBlock.Block.Header,
			Result:            er,
			Collection:        &collection,
			ChunkDataPack:     chdps[i],
			EndState:          chunk.EndState,
			TransactionOffset: offsetForChunk,
		}
	}

	require.Len(t, vcds, len(txs)+1) // +1 for system chunk

	for _, vcd := range vcds {
		var fault chmodels.ChunkFault
		if vcd.IsSystemChunk {
			_, fault, err = verifier.SystemChunkVerify(vcd)
		} else {
			_, fault, err = verifier.Verify(vcd)
		}
		assert.NoError(t, err)
		assert.Nil(t, fault)
	}

	return computationResult
}
