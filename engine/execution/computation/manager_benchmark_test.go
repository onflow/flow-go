package computation

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/onflow/flow-go/engine/execution/computation/committer"
	"github.com/onflow/flow-go/engine/execution/computation/computer"
	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/engine/execution/testutil"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool/entity"
	"github.com/onflow/flow-go/module/metrics"
	module "github.com/onflow/flow-go/module/mock"
	state_synchronization "github.com/onflow/flow-go/module/state_synchronization/mock"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/utils/unittest"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type testAccount struct {
	address    flow.Address
	privateKey flow.AccountPrivateKey
}

type testAccounts struct {
	accounts []testAccount
	seq      uint64
}

func createAccounts(b *testing.B, vm *fvm.VirtualMachine, ledger state.View, num int) *testAccounts {
	privateKeys, err := testutil.GenerateAccountPrivateKeys(num)
	require.NoError(b, err)

	addresses, err := testutil.CreateAccounts(vm, ledger, programs.NewEmptyPrograms(), privateKeys, chain)
	require.NoError(b, err)

	accs := &testAccounts{
		accounts: make([]testAccount, num),
	}
	for i := 0; i < num; i++ {
		accs.accounts[i] = testAccount{
			address:    addresses[i],
			privateKey: privateKeys[i],
		}
	}
	return accs
}

func mustFundAccounts(b *testing.B, vm *fvm.VirtualMachine, ledger state.View, execCtx fvm.Context, accs *testAccounts) {
	var err error
	for i, acc := range accs.accounts {
		transferTx := testutil.CreateTokenTransferTransaction(chain, 1_000_000, acc.address, chain.ServiceAddress())
		err = testutil.SignTransactionAsServiceAccount(transferTx, accs.seq, chain)
		require.NoError(b, err)
		accs.seq++

		tx := fvm.Transaction(transferTx, uint32(i))
		err = vm.Run(execCtx, tx, ledger, programs.NewEmptyPrograms())
		require.NoError(b, err)
		require.NoError(b, tx.Err)
	}
}

func BenchmarkComputeBlock(b *testing.B) {
	b.StopTimer()

	vm := fvm.NewVirtualMachine(fvm.NewInterpreterRuntime())

	chain := flow.Emulator.Chain()
	execCtx := fvm.NewContext(zerolog.Nop(), fvm.WithChain(chain))
	ledger := testutil.RootBootstrappedLedger(vm, execCtx)
	accs := createAccounts(b, vm, ledger, 1000)
	mustFundAccounts(b, vm, ledger, execCtx, accs)

	me := new(module.Local)
	me.On("NodeID").Return(flow.ZeroID)

	// TODO(rbtz): add real ledger
	blockComputer, err := computer.NewBlockComputer(vm, execCtx, metrics.NewNoopCollector(), trace.NewNoopTracer(), zerolog.Nop(), committer.NewNoopViewCommitter())
	require.NoError(b, err)

	programsCache, err := NewProgramsCache(1000)
	require.NoError(b, err)

	eds := new(state_synchronization.ExecutionDataService)
	eds.On("Add", mock.Anything, mock.Anything).Return(flow.ZeroID, nil, nil)

	eCache := new(state_synchronization.ExecutionDataCIDCache)
	eCache.On("Insert", mock.AnythingOfType("*flow.Header"), mock.AnythingOfType("state_synchronization.BlobTree"))

	engine := &Manager{
		blockComputer: blockComputer,
		tracer:        trace.NewNoopTracer(),
		me:            me,
		programsCache: programsCache,
		eds:           eds,
		edCache:       eCache,
	}

	view := delta.NewView(ledger.Get)
	blockView := view.NewChild()

	b.SetParallelism(1)

	parentBlock := &flow.Block{
		Header:  &flow.Header{},
		Payload: &flow.Payload{},
	}
	for _, cols := range []int{1, 4, 16} {
		for _, txes := range []int{16, 32, 64, 128} {
			cols := cols
			txes := txes
			b.Run(fmt.Sprintf("%d/cols/%d/txes", cols, txes), func(b *testing.B) {
				b.StopTimer()
				b.ResetTimer()

				var elapsed time.Duration
				for i := 0; i < b.N; i++ {
					executableBlock := createBlock(b, parentBlock, accs, cols, txes)
					parentBlock = executableBlock.Block

					b.StartTimer()
					start := time.Now()
					res, err := engine.ComputeBlock(context.Background(), executableBlock, blockView)
					elapsed += time.Since(start)
					b.StopTimer()

					require.NoError(b, err)
					for j, r := range res.TransactionResults {
						// skip system transactions
						if j >= cols*txes {
							break
						}
						require.Emptyf(b, r.ErrorMessage, "Transaction %d failed", j)
					}
				}
				totalTxes := int64(cols) * int64(txes) * int64(b.N)
				b.ReportMetric(float64(elapsed.Nanoseconds()/totalTxes/int64(time.Microsecond)), "us/tx")
			})
		}
	}
}

func createBlock(b *testing.B, parentBlock *flow.Block, accs *testAccounts, colNum int, txNum int) *entity.ExecutableBlock {
	completeCollections := make(map[flow.Identifier]*entity.CompleteCollection, colNum)
	collections := make([]*flow.Collection, colNum)
	guarantees := make([]*flow.CollectionGuarantee, colNum)

	for c := 0; c < colNum; c++ {
		transactions := make([]*flow.TransactionBody, txNum)
		for t := 0; t < txNum; t++ {
			transactions[t] = createTokenTransferTransaction(b, accs)
		}

		collection := &flow.Collection{Transactions: transactions}
		guarantee := &flow.CollectionGuarantee{CollectionID: collection.ID()}

		collections[c] = collection
		guarantees[c] = guarantee
		completeCollections[guarantee.ID()] = &entity.CompleteCollection{
			Guarantee:    guarantee,
			Transactions: transactions,
		}
	}

	block := flow.Block{
		Header: flow.NewHeader(
			"",
			parentBlock.ID(),
			0,
			flow.ZeroID,
			time.Time{},
			parentBlock.Header.Height+1,
			nil,
			nil,
			flow.ZeroID,
			nil),
		Payload: &flow.Payload{
			Guarantees: guarantees,
		},
	}

	return &entity.ExecutableBlock{
		Block:               &block,
		CompleteCollections: completeCollections,
		StartState:          unittest.StateCommitmentPointerFixture(),
	}
}

func createTokenTransferTransaction(b *testing.B, accs *testAccounts) *flow.TransactionBody {
	var err error

	rnd := rand.Intn(len(accs.accounts))
	src := accs.accounts[rnd]
	dst := accs.accounts[(rnd+1)%len(accs.accounts)]

	tx := testutil.CreateTokenTransferTransaction(chain, 1, dst.address, src.address)
	tx.SetProposalKey(chain.ServiceAddress(), 0, accs.seq).
		SetGasLimit(1000).
		SetPayer(chain.ServiceAddress())
	accs.seq++

	err = testutil.SignPayload(tx, src.address, src.privateKey)
	require.NoError(b, err)

	err = testutil.SignEnvelope(tx, chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
	require.NoError(b, err)

	return tx
}
