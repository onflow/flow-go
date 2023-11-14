package evm_test

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/json"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/evm/stdlib"
	. "github.com/onflow/flow-go/fvm/evm/testutils"

	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestEVMRun(t *testing.T) {

	t.Parallel()

	t.Run("testing EVM.run (happy case)", func(t *testing.T) {
		RunWithTestBackend(t, func(backend types.Backend) {
			RunWithTestFlowEVMRootAddress(t, backend, func(rootAddr flow.Address) {
				RunWithDeployedContract(t, backend, rootAddr, func(testContract *TestContract) {
					RunWithEOATestAccount(t, backend, rootAddr, func(testAccount *EOATestAccount) {
						num := int64(12)
						chain := flow.Emulator.Chain()

						RunWithNewTestVM(t, chain, func(ctx fvm.Context, vm fvm.VM, snapshot snapshot.SnapshotTree) {
							code := []byte(fmt.Sprintf(
								`
                          import EVM from %s

                          access(all)
                          fun main(tx: [UInt8], coinbaseBytes: [UInt8; 20]) {
                              let coinbase = EVM.EVMAddress(bytes: coinbaseBytes)
                              EVM.run(tx: tx, coinbase: coinbase)
                          }
                        `,
								chain.ServiceAddress().HexWithPrefix(),
							))

							gasLimit := uint64(100_000)

							txBytes := testAccount.PrepareSignAndEncodeTx(t,
								testContract.DeployedAt.ToCommon(),
								testContract.MakeStoreCallData(t, big.NewInt(num)),
								big.NewInt(0),
								gasLimit,
								big.NewInt(0),
							)

							tx := cadence.NewArray(
								ConvertToCadence(txBytes),
							).WithType(stdlib.EVMTransactionBytesCadenceType)

							coinbase := cadence.NewArray(
								ConvertToCadence(testAccount.Address().Bytes()),
							).WithType(stdlib.EVMAddressBytesCadenceType)

							script := fvm.Script(code).WithArguments(
								json.MustEncode(tx),
								json.MustEncode(coinbase),
							)

							_, output, err := vm.Run(
								ctx,
								script,
								snapshot)
							require.NoError(t, err)
							require.NoError(t, output.Err)
						})
					})
				})
			})
		})
	})
}

func RunWithNewTestVM(t *testing.T, chain flow.Chain, f func(fvm.Context, fvm.VM, snapshot.SnapshotTree)) {
	opts := []fvm.Option{
		fvm.WithChain(chain),
		fvm.WithAuthorizationChecksEnabled(false),
		fvm.WithSequenceNumberCheckAndIncrementEnabled(false),
	}
	ctx := fvm.NewContext(opts...)

	vm := fvm.NewVirtualMachine()
	snapshotTree := snapshot.NewSnapshotTree(nil)

	baseBootstrapOpts := []fvm.BootstrapProcedureOption{
		fvm.WithInitialTokenSupply(unittest.GenesisTokenSupply),
		fvm.WithSetupEVMEnabled(true),
	}

	executionSnapshot, _, err := vm.Run(
		ctx,
		fvm.Bootstrap(unittest.ServiceAccountPublicKey, baseBootstrapOpts...),
		snapshotTree)
	require.NoError(t, err)

	snapshotTree = snapshotTree.Append(executionSnapshot)

	f(fvm.NewContextFromParent(ctx, fvm.WithEVMEnabled(true)), vm, snapshotTree)
}

// TODO: test with actual amount
func TestEVMAddressDeposit(t *testing.T) {

	t.Parallel()

	RunWithTestBackend(t, func(backend types.Backend) {
		RunWithTestFlowEVMRootAddress(t, backend, func(rootAddr flow.Address) {
			RunWithDeployedContract(t, backend, rootAddr, func(testContract *TestContract) {
				RunWithEOATestAccount(t, backend, rootAddr, func(testAccount *EOATestAccount) {
					chain := flow.Emulator.Chain()
					RunWithNewTestVM(t, chain, func(ctx fvm.Context, vm fvm.VM, snapshot snapshot.SnapshotTree) {

						code := []byte(fmt.Sprintf(
							`
                               import EVM from %s
                               import FlowToken from %s

                               access(all)
                               fun main() {
                                   let address = EVM.EVMAddress(
                                       bytes: [2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]
                                   )
                                   let vault <- FlowToken.createEmptyVault() as! @FlowToken.Vault
                                   address.deposit(from: <-vault)
                               }
                            `,
							chain.ServiceAddress().HexWithPrefix(),
							fvm.FlowTokenAddress(chain).HexWithPrefix(),
						))

						script := fvm.Script(code)

						executionSnapshot, output, err := vm.Run(
							ctx,
							script,
							snapshot)
						require.NoError(t, err)
						require.NoError(t, output.Err)

						// TODO:
						_ = executionSnapshot
					})
				})
			})
		})
	})
}
