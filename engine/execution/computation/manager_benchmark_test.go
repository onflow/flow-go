package computation

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/onflow/cadence/runtime"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution/computation/committer"
	"github.com/onflow/flow-go/engine/execution/computation/computer"
	"github.com/onflow/flow-go/engine/execution/testutil"
	"github.com/onflow/flow-go/fvm"
	reusableRuntime "github.com/onflow/flow-go/fvm/runtime"
	"github.com/onflow/flow-go/fvm/storage/derived"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	exedataprovider "github.com/onflow/flow-go/module/executiondatasync/provider"
	mocktracker "github.com/onflow/flow-go/module/executiondatasync/tracker/mock"
	"github.com/onflow/flow-go/module/mempool/entity"
	"github.com/onflow/flow-go/module/metrics"
	module "github.com/onflow/flow-go/module/mock"
	requesterunit "github.com/onflow/flow-go/module/state_synchronization/requester/unittest"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/utils/unittest"
)

type testAccount struct {
	address    flow.Address
	privateKey flow.AccountPrivateKey
}

type testAccounts struct {
	accounts []testAccount
	seq      uint64
}

func createAccounts(
	b *testing.B,
	vm fvm.VM,
	snapshotTree snapshot.SnapshotTree,
	num int,
) (
	snapshot.SnapshotTree,
	*testAccounts,
) {
	privateKeys, err := testutil.GenerateAccountPrivateKeys(num)
	require.NoError(b, err)

	snapshotTree, addresses, err := testutil.CreateAccounts(
		vm,
		snapshotTree,
		privateKeys,
		chain)
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
	return snapshotTree, accs
}

func mustFundAccounts(
	b *testing.B,
	vm fvm.VM,
	snapshotTree snapshot.SnapshotTree,
	execCtx fvm.Context,
	accs *testAccounts,
) snapshot.SnapshotTree {
	var err error
	for _, acc := range accs.accounts {
		transferTx := testutil.CreateTokenTransferTransaction(chain, 1_000_000, acc.address, chain.ServiceAddress())
		err = testutil.SignTransactionAsServiceAccount(transferTx, accs.seq, chain)
		require.NoError(b, err)
		accs.seq++

		executionSnapshot, output, err := vm.Run(
			execCtx,
			fvm.Transaction(transferTx, 0),
			snapshotTree)
		require.NoError(b, err)
		require.NoError(b, output.Err)
		snapshotTree = snapshotTree.Append(executionSnapshot)
	}

	return snapshotTree
}

func BenchmarkComputeBlock(b *testing.B) {
	b.StopTimer()
	b.SetParallelism(1)

	type benchmarkCase struct {
		numCollections               int
		numTransactionsPerCollection int
		maxConcurrency               int
	}

	for _, benchCase := range []benchmarkCase{
		{
			numCollections:               16,
			numTransactionsPerCollection: 128,
			maxConcurrency:               1,
		},
		{
			numCollections:               16,
			numTransactionsPerCollection: 128,
			maxConcurrency:               2,
		},
	} {
		b.Run(
			fmt.Sprintf(
				"%d/cols/%d/txes/%d/max-concurrency",
				benchCase.numCollections,
				benchCase.numTransactionsPerCollection,
				benchCase.maxConcurrency),
			func(b *testing.B) {
				benchmarkComputeBlock(
					b,
					benchCase.numCollections,
					benchCase.numTransactionsPerCollection,
					benchCase.maxConcurrency)
			})
	}
}

func benchmarkComputeBlock(
	b *testing.B,
	numCollections int,
	numTransactionsPerCollection int,
	maxConcurrency int,
) {
	tracer, err := trace.NewTracer(zerolog.Nop(), "", "", 4)
	require.NoError(b, err)

	vm := fvm.NewVirtualMachine()

	const chainID = flow.Emulator
	execCtx := fvm.NewContext(
		fvm.WithChain(chainID.Chain()),
		fvm.WithAccountStorageLimit(true),
		fvm.WithTransactionFeesEnabled(true),
		fvm.WithTracer(tracer),
		fvm.WithReusableCadenceRuntimePool(
			reusableRuntime.NewReusableCadenceRuntimePool(
				ReusableCadenceRuntimePoolSize,
				runtime.Config{},
			)),
	)
	snapshotTree := testutil.RootBootstrappedLedger(
		vm,
		execCtx,
		fvm.WithAccountCreationFee(fvm.DefaultAccountCreationFee),
		fvm.WithMinimumStorageReservation(fvm.DefaultMinimumStorageReservation),
		fvm.WithTransactionFee(fvm.DefaultTransactionFees),
		fvm.WithStorageMBPerFLOW(fvm.DefaultStorageMBPerFLOW),
	)
	snapshotTree, accs := createAccounts(b, vm, snapshotTree, 1000)
	snapshotTree = mustFundAccounts(b, vm, snapshotTree, execCtx, accs)

	me := new(module.Local)
	me.On("NodeID").Return(flow.ZeroID)
	me.On("Sign", mock.Anything, mock.Anything).Return(nil, nil)
	me.On("SignFunc", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil)

	bservice := requesterunit.MockBlobService(blockstore.NewBlockstore(dssync.MutexWrap(datastore.NewMapDatastore())))
	trackerStorage := mocktracker.NewMockStorage()

	prov := exedataprovider.NewProvider(
		zerolog.Nop(),
		metrics.NewNoopCollector(),
		execution_data.DefaultSerializer,
		bservice,
		trackerStorage,
	)

	// TODO(rbtz): add real ledger
	blockComputer, err := computer.NewBlockComputer(
		vm,
		execCtx,
		metrics.NewNoopCollector(),
		tracer,
		zerolog.Nop(),
		committer.NewNoopViewCommitter(),
		me,
		prov,
		nil,
		testutil.ProtocolStateWithSourceFixture(nil),
		maxConcurrency)
	require.NoError(b, err)

	derivedChainData, err := derived.NewDerivedChainData(
		derived.DefaultDerivedDataCacheSize)
	require.NoError(b, err)

	engine := &Manager{
		blockComputer:    blockComputer,
		derivedChainData: derivedChainData,
	}

	parentBlock := &flow.Block{
		Header:  &flow.Header{},
		Payload: &flow.Payload{},
	}

	b.StopTimer()
	b.ResetTimer()

	var elapsed time.Duration
	for i := 0; i < b.N; i++ {
		executableBlock := createBlock(
			b,
			parentBlock,
			accs,
			numCollections,
			numTransactionsPerCollection)
		parentBlock = executableBlock.Block

		b.StartTimer()
		start := time.Now()
		res, err := engine.ComputeBlock(
			context.Background(),
			unittest.IdentifierFixture(),
			executableBlock,
			snapshotTree)
		elapsed += time.Since(start)
		b.StopTimer()

		require.NoError(b, err)
		for _, snapshot := range res.AllExecutionSnapshots() {
			snapshotTree = snapshotTree.Append(snapshot)
		}

		for j, r := range res.AllTransactionResults() {
			// skip system transactions
			if j >= numCollections*numTransactionsPerCollection {
				break
			}
			require.Emptyf(b, r.ErrorMessage, "Transaction %d failed", j)
		}
	}
	totalTxes := int64(numCollections) * int64(numTransactionsPerCollection) * int64(b.N)
	b.ReportMetric(float64(elapsed.Nanoseconds()/totalTxes/int64(time.Microsecond)), "us/tx")
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
		Header: &flow.Header{
			ParentID: parentBlock.ID(),
			View:     parentBlock.Header.Height + 1,
		},
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
		SetComputeLimit(1000).
		SetPayer(chain.ServiceAddress())
	accs.seq++

	err = testutil.SignPayload(tx, src.address, src.privateKey)
	require.NoError(b, err)

	err = testutil.SignEnvelope(tx, chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
	require.NoError(b, err)

	return tx
}
