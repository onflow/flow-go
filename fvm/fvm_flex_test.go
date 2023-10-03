package fvm_test

import (
	"math"
	"math/big"
	"testing"

	"github.com/onflow/atree"
	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/flex/stdlib/emulator"
	"github.com/onflow/flow-go/fvm/flex/testutils"
	"github.com/onflow/flow-go/fvm/storage"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	storageutils "github.com/onflow/flow-go/fvm/storage/testutils"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
)

func WithBootstrappedTestVM(
	t *testing.T,
	chainID flow.ChainID,
	f func(
		flow.Chain,
		fvm.VM,
		*snapshot.SnapshotTree,
	),
) {
	chain, vm := createChainAndVm(chainID)

	ctx := fvm.NewContext(
		fvm.WithChain(chain),
		fvm.WithAuthorizationChecksEnabled(false),
		fvm.WithSequenceNumberCheckAndIncrementEnabled(false),
	)

	snapshotTree := snapshot.NewSnapshotTree(nil)

	baseBootstrapOpts := []fvm.BootstrapProcedureOption{
		fvm.WithInitialTokenSupply(unittest.GenesisTokenSupply),
	}

	executionSnapshot, _, err := vm.Run(
		ctx,
		fvm.Bootstrap(unittest.ServiceAccountPublicKey, baseBootstrapOpts...),
		snapshotTree)
	require.NoError(t, err)

	snapshotTree = snapshotTree.Append(executionSnapshot)
	f(chain, vm, &snapshotTree)
}

func TestWithFlexEnabled_ContractInteraction(t *testing.T) {

	chainID := flow.Emulator
	flexRoot := emulator.FlexRootAccountAddress
	var snapshotTree snapshot.SnapshotTree
	WithBootstrappedTestVM(t, chainID, func(chain flow.Chain, vm fvm.VM, tree *snapshot.SnapshotTree) {
		ledger := newLedger(tree)
		snapshotTree = *tree
		testutils.RunWithDeployedContract(t, ledger, flexRoot, func(testContract *testutils.TestContract) {
			testutils.RunWithEOATestAccount(t, ledger, flexRoot, func(testAccount *testutils.EOATestAccount) {
				com, err := ledger.Commit()
				require.NoError(t, err)
				snapshotTree = snapshotTree.Append(com)

				// test storing a value
				num := int64(12)

				// create ctx with flex enabled
				ctx := fvm.NewContext(
					fvm.WithChain(chain),
					fvm.WithFlexEnabled(true),
					fvm.WithAuthorizationChecksEnabled(false),
					fvm.WithSequenceNumberCheckAndIncrementEnabled(false),
				)

				txScript := []byte(`
					transaction(tx: [UInt8], coinbaseBytes: [UInt8; 20]) {
						prepare(signer: AuthAccount) {
						}
						execute {
							let coinbase = Flex.FlexAddress(bytes: coinbaseBytes)
							assert(Flex.run(tx: tx, coinbase: coinbase), message: "tx execution failed")
						}
					}
			`)

				storeTxBytes := testAccount.PrepareSignAndEncodeTx(t,
					testContract.DeployedAt,
					testContract.MakeStoreCallData(t, big.NewInt(num)),
					big.NewInt(0),
					math.MaxUint64,
					big.NewInt(1),
				)

				encodedInnerTx, err := jsoncdc.Encode(
					cadence.NewArray(
						testutils.ConvertToCadence(storeTxBytes),
					),
				)
				require.NoError(t, err)

				encodedCoinbase, err := jsoncdc.Encode(
					cadence.NewArray(
						testutils.ConvertToCadence(testAccount.FlexAddress().Bytes()),
					),
				)
				require.NoError(t, err)

				storeTxBody := flow.NewTransactionBody().
					SetScript(txScript).
					AddAuthorizer(chain.ServiceAddress()).
					AddArgument(encodedInnerTx).
					AddArgument(encodedCoinbase)

				executionSnapshot, output, err := vm.Run(
					ctx,
					fvm.Transaction(storeTxBody, 0),
					snapshotTree)
				require.NoError(t, err)

				// transaction should pass
				require.NoError(t, output.Err)

				snapshotTree = snapshotTree.Append(executionSnapshot)

				// test retriveing a value
				retrieveTxBytes := testAccount.PrepareSignAndEncodeTx(t,
					testContract.DeployedAt,
					testContract.MakeRetrieveCallData(t),
					big.NewInt(0),
					math.MaxUint64,
					big.NewInt(1),
				)

				encodedInnerTx, err = jsoncdc.Encode(
					cadence.NewArray(
						testutils.ConvertToCadence(retrieveTxBytes),
					),
				)
				require.NoError(t, err)

				encodedCoinbase, err = jsoncdc.Encode(
					cadence.NewArray(
						testutils.ConvertToCadence(testAccount.FlexAddress().Bytes()),
					),
				)
				require.NoError(t, err)

				script := []byte(`
					pub fun main(tx: [UInt8], coinbaseBytes: [UInt8; 20]): Bool {
						let coinbase = Flex.FlexAddress(bytes: coinbaseBytes)
						return Flex.run(tx: tx, coinbase: coinbase)
					}
			`)

				retriveScriptBody := fvm.Script(script).
					WithArguments(encodedInnerTx, encodedCoinbase)

				_, output, err = vm.Run(
					ctx,
					retriveScriptBody,
					snapshotTree)
				require.NoError(t, err)

				// transaction should pass
				require.NoError(t, output.Err)

				require.Equal(t, output.Value, true)
				// TODO check the actual value
				// require.Equal(t, output.Value, num)
			})
		})
	})
}

type ledger struct {
	tree     *snapshot.SnapshotTree
	txState  storage.Transaction
	accounts environment.Accounts
}

var _ atree.Ledger = &ledger{}

func newLedger(tree *snapshot.SnapshotTree) *ledger {
	txnState := storageutils.NewSimpleTransaction(tree)
	accounts := environment.NewAccounts(txnState)
	return &ledger{
		tree:     tree,
		txState:  txnState,
		accounts: accounts,
	}
}

func (l *ledger) Commit() (*snapshot.ExecutionSnapshot, error) {
	err := l.txState.Finalize()
	if err != nil {
		return nil, err
	}
	return l.txState.Commit()
}

func (l *ledger) GetValue(owner, key []byte) (value []byte, err error) {
	return l.accounts.GetValue(flow.RegisterID{
		Owner: string(owner),
		Key:   string(key),
	})
}

func (l *ledger) SetValue(owner, key, value []byte) (err error) {
	return l.accounts.SetValue(flow.RegisterID{
		Owner: string(owner),
		Key:   string(key),
	}, value)
}

func (l *ledger) ValueExists(owner, key []byte) (exists bool, err error) {
	data, err := l.GetValue(owner, key)
	return len(data) > 0, err
}

func (l *ledger) AllocateStorageIndex(owner []byte) (atree.StorageIndex, error) {
	return l.accounts.AllocateStorageIndex(flow.Address(owner))
}
