package computation

import (
	"context"
	"fmt"
	"testing"

	"github.com/onflow/cadence"

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

	noTxFee, err := cadence.NewUFix64("0.0")
	require.NoError(t, err)

	t.Run("empty block", func(t *testing.T) {
		executeBlockAndVerify(t,
			[][]*flow.TransactionBody{},
			fvm.DefaultTransactionFees,
			fvm.DefaultMinimumStorageReservation)
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
		}, noTxFee, fvm.DefaultMinimumStorageReservation)

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
		}, noTxFee, fvm.DefaultMinimumStorageReservation)

		// ensure event is emitted
		require.Empty(t, cr.TransactionResults[0].ErrorMessage)
		require.Empty(t, cr.TransactionResults[1].ErrorMessage)
		require.Empty(t, cr.TransactionResults[2].ErrorMessage)
		require.Empty(t, cr.TransactionResults[3].ErrorMessage)
		require.Len(t, cr.Events[0], 2)
		require.Equal(t, flow.EventType(fmt.Sprintf("A.%s.Foo.FooEvent", chain.ServiceAddress())), cr.Events[0][1].Type)
	})

	t.Run("with failed storage limit", func(t *testing.T) {

		accountPrivKey, createAccountTx := testutil.CreateAccountCreationTransaction(t, chain)

		// this should return the address of newly created account
		accountAddress, err := chain.AddressAtIndex(5)
		require.NoError(t, err)

		err = testutil.SignTransactionAsServiceAccount(createAccountTx, 0, chain)
		require.NoError(t, err)

		addKeyTx := testutil.CreateAddAnAccountKeyMultipleTimesTransaction(t, &accountPrivKey, 100).AddAuthorizer(accountAddress)
		err = testutil.SignTransaction(addKeyTx, accountAddress, accountPrivKey, 0)
		require.NoError(t, err)

		minimumStorage, err := cadence.NewUFix64("0.00007761")
		require.NoError(t, err)

		cr := executeBlockAndVerify(t, [][]*flow.TransactionBody{
			{
				createAccountTx,
			},
			{
				addKeyTx,
			},
		}, fvm.DefaultTransactionFees, minimumStorage)

		// ensure only events from the first transaction is emitted
		require.Len(t, cr.Events[0], 10)
		require.Len(t, cr.Events[1], 0)
		// storage limit error
		assert.Contains(t, cr.TransactionResults[1].ErrorMessage, "Error Code: 1103")
	})

	t.Run("with failed transaction fee deduction", func(t *testing.T) {
		accountPrivKey, createAccountTx := testutil.CreateAccountCreationTransaction(t, chain)

		// this should return the address of newly created account
		accountAddress, err := chain.AddressAtIndex(5)
		require.NoError(t, err)

		err = testutil.SignTransactionAsServiceAccount(createAccountTx, 0, chain)
		require.NoError(t, err)

		spamTx := &flow.TransactionBody{
			Script: []byte(`
			transaction {
				prepare() {}
				execute {
					var s: Int256 = 1024102410241024
					var i = 0
					var a = Int256(7)
					var b = Int256(5)
					var c = Int256(2)
					while i < 150000 {
						s = s * a
						s = s / b
						s = s / c
						i = i + 1
					}
					log(i)
				}
			}`),
		}

		err = testutil.SignTransaction(spamTx, accountAddress, accountPrivKey, 0)
		require.NoError(t, err)

		txFee, err := cadence.NewUFix64("0.01")
		require.NoError(t, err)

		cr := executeBlockAndVerify(t, [][]*flow.TransactionBody{
			{
				createAccountTx,
				spamTx,
			},
		}, txFee, fvm.DefaultMinimumStorageReservation)

		// ensure only events from the first transaction is emitted
		require.Len(t, cr.Events[0], 10)
		require.Len(t, cr.Events[1], 0)
		// tx fee error
		assert.Contains(t, cr.TransactionResults[1].ErrorMessage, "Error Code: 1109")
	})

}

func executeBlockAndVerify(t *testing.T,
	txs [][]*flow.TransactionBody,
	txFees cadence.UFix64,
	minStorageBalance cadence.UFix64) *execution.ComputationResult {
	rt := fvm.NewInterpreterRuntime()
	vm := fvm.NewVirtualMachine(rt)

	logger := zerolog.Nop()

	fvmContext := fvm.NewContext(logger,
		fvm.WithChain(chain),
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
		fvm.WithMinimumStorageReservation(minStorageBalance),
		fvm.WithTransactionFee(txFees),
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

	verifier := chunks.NewChunkVerifier(vm, fvmContext)

	vcds := make([]*verification.VerifiableChunkData, er.Chunks.Len())

	for i, chunk := range er.Chunks {
		isSystemChunk := i == er.Chunks.Len()-1
		offsetForChunk, err := fetcher.TransactionOffsetForChunk(er.Chunks, chunk.Index)
		require.NoError(t, err)

		vcds[i] = &verification.VerifiableChunkData{
			IsSystemChunk:     isSystemChunk,
			Chunk:             chunk,
			Header:            executableBlock.Block.Header,
			Result:            er,
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
