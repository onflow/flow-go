package load_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/ccf"
	convert2 "github.com/onflow/flow-emulator/convert"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	sdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/crypto"

	cadenceCommon "github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/engine/execution/computation"
	"github.com/onflow/flow-go/engine/execution/testutil"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/environment"
	envMock "github.com/onflow/flow-go/fvm/environment/mock"
	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/integration/benchmark/account"
	"github.com/onflow/flow-go/integration/benchmark/common"
	"github.com/onflow/flow-go/integration/benchmark/load"
	"github.com/onflow/flow-go/integration/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestLoadTypes(t *testing.T) {

	log := zerolog.New(zerolog.NewTestWriter(t))

	evmLoad := load.NewEVMTransferLoad(log)
	// don't create that many accounts for the test
	evmLoad.PreCreateEOAAccounts = 20

	evmBatchLoad := load.NewEVMBatchTransferLoad(log)
	// don't create that many accounts for the test
	evmBatchLoad.PreCreateEOAAccounts = 20

	loads := []load.Load{
		load.CompHeavyLoad,
		load.EventHeavyLoad,
		load.LedgerHeavyLoad,
		load.ExecDataHeavyLoad,
		load.NewTokenTransferLoad(),
		load.NewTokenTransferMultiLoad(),
		load.NewAddKeysLoad(),
		evmLoad,
		evmBatchLoad,
		load.NewCreateAccountLoad(),
	}

	for _, l := range loads {
		t.Run(string(l.Type()), testLoad(log, l))
	}
}

func testLoad(log zerolog.Logger, l load.Load) func(t *testing.T) {

	return func(t *testing.T) {

		chain := flow.Benchnet.Chain()

		vm, ctx, snapshotTree := bootstrapVM(t, chain)
		testSnapshotTree := &testSnapshotTree{snapshot: snapshotTree}

		blockProvider := noopReferenceBlockProvider{}
		transactionSender := &testTransactionSender{
			t:        t,
			log:      log.With().Str("component", "testTransactionSender").Logger(),
			vm:       vm,
			ctx:      ctx,
			snapshot: testSnapshotTree,
		}
		accountLoader := &TestAccountLoader{
			ctx:      ctx,
			vm:       vm,
			snapshot: testSnapshotTree,
		}

		serviceAccount, err := accountLoader.Load(sdk.ServiceAddress(sdk.ChainID(chain.ChainID())), unittest.ServiceAccountPrivateKey.PrivateKey, unittest.ServiceAccountPrivateKey.HashAlgo)
		require.NoError(t, err)

		err = account.EnsureAccountHasKeys(log, serviceAccount, 50, blockProvider, transactionSender)
		require.NoError(t, err)

		err = account.ReloadAccount(accountLoader, serviceAccount)
		require.NoError(t, err)

		accountProvider, err := account.SetupProvider(
			log,
			context.Background(),
			100,
			10_000_000_000,
			blockProvider,
			serviceAccount,
			transactionSender,
			chain,
		)
		require.NoError(t, err)

		lc := load.LoadContext{
			ChainID:                chain.ChainID(),
			AccountProvider:        accountProvider,
			ReferenceBlockProvider: blockProvider,
			TransactionSender:      transactionSender,
			WorkerContext: load.WorkerContext{
				WorkerID: 0,
			},
			Proposer: serviceAccount,
		}

		err = l.Setup(log, lc)
		require.NoError(t, err)
		for i := 0; i < 100; i++ {
			err = l.Load(log, lc)
			require.NoError(t, err)
		}
	}
}

func bootstrapVM(t *testing.T, chain flow.Chain) (*fvm.VirtualMachine, fvm.Context, snapshot.SnapshotTree) {
	source := testutil.EntropyProviderFixture(nil)

	blocks := new(envMock.Blocks)
	block1 := unittest.BlockFixture()
	blocks.On("ByHeightFrom",
		block1.Header.Height,
		block1.Header,
	).Return(block1.Header, nil)

	opts := computation.DefaultFVMOptions(chain.ChainID(), false, false)
	opts = append(opts,
		fvm.WithTransactionFeesEnabled(true),
		fvm.WithAccountStorageLimit(true),
		fvm.WithContractDeploymentRestricted(false),
		fvm.WithEntropyProvider(source),
		fvm.WithBlocks(blocks),
		fvm.WithBlockHeader(block1.Header),
	)

	ctx := fvm.NewContext(opts...)

	vm := fvm.NewVirtualMachine()
	snapshotTree := snapshot.NewSnapshotTree(nil)
	bootstrapOpts := []fvm.BootstrapProcedureOption{
		fvm.WithInitialTokenSupply(unittest.GenesisTokenSupply),
		fvm.WithAccountCreationFee(fvm.DefaultAccountCreationFee),
		fvm.WithMinimumStorageReservation(fvm.DefaultMinimumStorageReservation),
		fvm.WithTransactionFee(fvm.DefaultTransactionFees),
		fvm.WithStorageMBPerFLOW(fvm.DefaultStorageMBPerFLOW),
	}

	executionSnapshot, _, err := vm.Run(
		ctx,
		fvm.Bootstrap(unittest.ServiceAccountPublicKey, bootstrapOpts...),
		snapshotTree)
	require.NoError(t, err)
	snapshotTree = snapshotTree.Append(executionSnapshot)

	return vm, ctx, snapshotTree
}

type noopReferenceBlockProvider struct{}

func (n noopReferenceBlockProvider) ReferenceBlockID() sdk.Identifier {
	return sdk.EmptyID
}

var _ common.ReferenceBlockProvider = noopReferenceBlockProvider{}

type testTransactionSender struct {
	t        *testing.T
	log      zerolog.Logger
	vm       *fvm.VirtualMachine
	ctx      fvm.Context
	snapshot *testSnapshotTree
}

var _ common.TransactionSender = (*testTransactionSender)(nil)

func (t *testTransactionSender) Send(tx *sdk.Transaction) (sdk.TransactionResult, error) {
	txBody :=
		flow.NewTransactionBody().
			SetScript(tx.Script).
			SetReferenceBlockID(convert.IDFromSDK(tx.ReferenceBlockID)).
			SetComputeLimit(tx.GasLimit).
			SetProposalKey(
				flow.BytesToAddress(tx.ProposalKey.Address.Bytes()),
				uint64(tx.ProposalKey.KeyIndex),
				tx.ProposalKey.SequenceNumber,
			).
			SetPayer(flow.BytesToAddress(tx.Payer.Bytes()))

	for _, auth := range tx.Authorizers {
		txBody.AddAuthorizer(flow.BytesToAddress(auth.Bytes()))
	}
	for _, arg := range tx.Arguments {
		txBody.AddArgument(arg)
	}
	for _, sig := range tx.PayloadSignatures {
		txBody.AddPayloadSignature(
			flow.BytesToAddress(sig.Address.Bytes()),
			uint64(sig.KeyIndex),
			sig.Signature,
		)
	}
	for _, sig := range tx.EnvelopeSignatures {
		txBody.AddEnvelopeSignature(
			flow.BytesToAddress(sig.Address.Bytes()),
			uint64(sig.KeyIndex),
			sig.Signature,
		)
	}

	require.Equal(t.t, string(tx.PayloadMessage()), string(txBody.PayloadMessage()))
	require.Equal(t.t, string(tx.EnvelopeMessage()), string(txBody.EnvelopeMessage()))

	proc := fvm.Transaction(txBody, 0)

	t.snapshot.Lock()
	defer t.snapshot.Unlock()

	executionSnapshot, result, err := t.vm.Run(t.ctx, proc, t.snapshot)
	if err != nil {
		return sdk.TransactionResult{}, err
	}
	// Update the snapshot
	t.snapshot.Append(executionSnapshot)

	// temporarily hardcode the weights as they are not confirmed yet
	executionEffortWeights := meter.ExecutionEffortWeights{
		cadenceCommon.ComputationKindStatement:          314,
		cadenceCommon.ComputationKindLoop:               314,
		cadenceCommon.ComputationKindFunctionInvocation: 314,
		environment.ComputationKindGetValue:             162,
		environment.ComputationKindCreateAccount:        567534,
		environment.ComputationKindSetValue:             153,
		environment.ComputationKindEVMGasUsage:          13,
	}

	computationUsed := executionEffortWeights.ComputationFromIntensities(result.ComputationIntensities)
	t.log.Debug().Uint64("computation", computationUsed).Msg("Transaction applied")

	sdkResult := sdk.TransactionResult{
		Status:        sdk.TransactionStatusSealed,
		Error:         result.Err,
		BlockID:       sdk.EmptyID,
		BlockHeight:   0,
		TransactionID: convert2.FlowIdentifierToSDK(txBody.ID()),
		CollectionID:  sdk.EmptyID,
	}

	for _, event := range result.Events {
		decoded, err := ccf.Decode(nil, event.Payload)
		if err != nil {
			return sdkResult, fmt.Errorf("error decoding event payload: %w", err)
		}

		sdkResult.Events = append(sdkResult.Events, sdk.Event{
			Type:             string(event.Type),
			TransactionID:    sdk.Identifier{},
			TransactionIndex: 0,
			EventIndex:       int(event.EventIndex),
			Value:            decoded.(cadence.Event),
			Payload:          event.Payload,
		})
	}

	if result.Err != nil {
		return sdkResult, common.NewTransactionError(result.Err)
	}

	return sdkResult, nil
}

type TestAccountLoader struct {
	ctx      fvm.Context
	vm       *fvm.VirtualMachine
	snapshot *testSnapshotTree
}

var _ account.Loader = (*TestAccountLoader)(nil)

func (t *TestAccountLoader) Load(
	address sdk.Address,
	privateKey crypto.PrivateKey,
	hashAlgo crypto.HashAlgorithm) (*account.FlowAccount, error) {
	wrapErr := func(err error) error {
		return fmt.Errorf("error while loading account: %w", err)
	}

	t.snapshot.Lock()
	defer t.snapshot.Unlock()

	acc, err := t.vm.GetAccount(t.ctx, flow.ConvertAddress(address), t.snapshot)
	if err != nil {
		return nil, wrapErr(err)
	}

	keys := make([]sdk.AccountKey, 0, len(acc.Keys))
	for _, key := range acc.Keys {
		keys = append(keys, sdk.AccountKey{
			Index:          key.Index,
			PublicKey:      key.PublicKey,
			SigAlgo:        key.SignAlgo,
			HashAlgo:       key.HashAlgo,
			Weight:         key.Weight,
			SequenceNumber: key.SeqNumber,
			Revoked:        key.Revoked,
		})
	}

	return account.New(address, privateKey, hashAlgo, keys)
}

type testSnapshotTree struct {
	snapshot snapshot.SnapshotTree
	sync.Mutex
}

func (t *testSnapshotTree) Get(id flow.RegisterID) (flow.RegisterValue, error) {
	return t.snapshot.Get(id)
}

var _ snapshot.StorageSnapshot = (*testSnapshotTree)(nil)

func (t *testSnapshotTree) Append(snapshot *snapshot.ExecutionSnapshot) {
	t.snapshot = t.snapshot.Append(snapshot)
}
