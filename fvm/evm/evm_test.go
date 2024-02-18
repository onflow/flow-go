package evm_test

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/json"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution/testutil"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/crypto"
	envMock "github.com/onflow/flow-go/fvm/environment/mock"
	"github.com/onflow/flow-go/fvm/evm"
	"github.com/onflow/flow-go/fvm/evm/stdlib"
	. "github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestEVMRun(t *testing.T) {
	t.Parallel()

	t.Run("testing EVM.run (happy case)", func(t *testing.T) {
		t.Parallel()
		chain := flow.Emulator.Chain()
		RunWithNewEnvironment(t,
			chain, func(
				ctx fvm.Context,
				vm fvm.VM,
				snapshot snapshot.SnapshotTree,
				testContract *TestContract,
				testAccount *EOATestAccount,
			) {
				sc := systemcontracts.SystemContractsForChain(chain.ChainID())
				code := []byte(fmt.Sprintf(
					`
					import EVM from %s

					access(all)
					fun main(tx: [UInt8], coinbaseBytes: [UInt8; 20]) {
						let coinbase = EVM.EVMAddress(bytes: coinbaseBytes)
						EVM.run(tx: tx, coinbase: coinbase)
					}
					`,
					sc.EVMContract.Address.HexWithPrefix(),
				))

				num := int64(12)
				txBytes := testAccount.PrepareSignAndEncodeTx(t,
					testContract.DeployedAt.ToCommon(),
					testContract.MakeCallData(t, "store", big.NewInt(num)),
					big.NewInt(0),
					uint64(100_000),
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
}

func TestEVMAddressDeposit(t *testing.T) {
	t.Parallel()
	chain := flow.Emulator.Chain()
	sc := systemcontracts.SystemContractsForChain(chain.ChainID())
	RunWithNewEnvironment(t,
		chain, func(
			ctx fvm.Context,
			vm fvm.VM,
			snapshot snapshot.SnapshotTree,
			testContract *TestContract,
			testAccount *EOATestAccount,
		) {
			code := []byte(fmt.Sprintf(
				`
				import EVM from %s
				import FlowToken from %s

				access(all)
				fun main() {
					let admin = getAuthAccount(%s)
						.borrow<&FlowToken.Administrator>(from: /storage/flowTokenAdmin)!
					let minter <- admin.createNewMinter(allowedAmount: 1.23)
					let vault <- minter.mintTokens(amount: 1.23)
					destroy minter

					let cadenceOwnedAccount <- EVM.createCadenceOwnedAccount()
					cadenceOwnedAccount.deposit(from: <-vault)
					destroy cadenceOwnedAccount
				}
                `,
				sc.EVMContract.Address.HexWithPrefix(),
				sc.FlowToken.Address.HexWithPrefix(),
				sc.FlowServiceAccount.Address.HexWithPrefix(),
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
}

func TestCOAWithdraw(t *testing.T) {

	t.Parallel()
	chain := flow.Emulator.Chain()
	sc := systemcontracts.SystemContractsForChain(chain.ChainID())
	RunWithNewEnvironment(t,
		chain, func(
			ctx fvm.Context,
			vm fvm.VM,
			snapshot snapshot.SnapshotTree,
			testContract *TestContract,
			testAccount *EOATestAccount,
		) {
			code := []byte(fmt.Sprintf(
				`
				import EVM from %s
				import FlowToken from %s

				access(all)
				fun main(): UFix64 {
					let admin = getAuthAccount(%s)
						.borrow<&FlowToken.Administrator>(from: /storage/flowTokenAdmin)!
					let minter <- admin.createNewMinter(allowedAmount: 2.34)
					let vault <- minter.mintTokens(amount: 2.34)
					destroy minter

					let cadenceOwnedAccount <- EVM.createCadenceOwnedAccount()
					cadenceOwnedAccount.deposit(from: <-vault)

					let bal = EVM.Balance(0)
					bal.setFLOW(flow: 1.23)
					let vault2 <- cadenceOwnedAccount.withdraw(balance: bal)
					let balance = vault2.balance
					destroy cadenceOwnedAccount
					destroy vault2

					return balance
				}
				`,
				sc.EVMContract.Address.HexWithPrefix(),
				sc.FlowToken.Address.HexWithPrefix(),
				sc.FlowServiceAccount.Address.HexWithPrefix(),
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
}

func TestCadenceOwnedAccountDeploy(t *testing.T) {
	t.Parallel()
	chain := flow.Emulator.Chain()
	sc := systemcontracts.SystemContractsForChain(chain.ChainID())
	RunWithNewEnvironment(t,
		chain, func(
			ctx fvm.Context,
			vm fvm.VM,
			snapshot snapshot.SnapshotTree,
			testContract *TestContract,
			testAccount *EOATestAccount,
		) {
			code := []byte(fmt.Sprintf(
				`
				import EVM from %s
				import FlowToken from %s

				access(all)
				fun main(): [UInt8; 20] {
					let admin = getAuthAccount(%s)
						.borrow<&FlowToken.Administrator>(from: /storage/flowTokenAdmin)!
					let minter <- admin.createNewMinter(allowedAmount: 2.34)
					let vault <- minter.mintTokens(amount: 2.34)
					destroy minter

					let cadenceOwnedAccount <- EVM.createCadenceOwnedAccount()
					cadenceOwnedAccount.deposit(from: <-vault)

					let address = cadenceOwnedAccount.deploy(
						code: [],
						gasLimit: 53000,
						value: EVM.Balance(attoflow: 1230000000000000000)
					)
					destroy cadenceOwnedAccount
					return address.bytes
				}
                `,
				sc.EVMContract.Address.HexWithPrefix(),
				sc.FlowToken.Address.HexWithPrefix(),
				sc.FlowServiceAccount.Address.HexWithPrefix(),
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
}

func TestCadenceArch(t *testing.T) {
	t.Parallel()

	t.Run("testing calling Cadence arch - flow block height (happy case)", func(t *testing.T) {
		chain := flow.Emulator.Chain()
		sc := systemcontracts.SystemContractsForChain(chain.ChainID())
		RunWithNewEnvironment(t,
			chain, func(
				ctx fvm.Context,
				vm fvm.VM,
				snapshot snapshot.SnapshotTree,
				testContract *TestContract,
				testAccount *EOATestAccount,
			) {
				code := []byte(fmt.Sprintf(
					`
					import EVM from %s

					access(all)
					fun main(tx: [UInt8], coinbaseBytes: [UInt8; 20]) {
						let coinbase = EVM.EVMAddress(bytes: coinbaseBytes)
						EVM.run(tx: tx, coinbase: coinbase)
					}
                    `,
					sc.EVMContract.Address.HexWithPrefix(),
				))
				innerTxBytes := testAccount.PrepareSignAndEncodeTx(t,
					testContract.DeployedAt.ToCommon(),
					testContract.MakeCallData(t, "verifyArchCallToFlowBlockHeight", uint64(ctx.BlockHeader.Height)),
					big.NewInt(0),
					uint64(10_000_000),
					big.NewInt(0),
				)
				script := fvm.Script(code).WithArguments(
					json.MustEncode(
						cadence.NewArray(
							ConvertToCadence(innerTxBytes),
						).WithType(stdlib.EVMTransactionBytesCadenceType),
					),
					json.MustEncode(
						cadence.NewArray(
							ConvertToCadence(testAccount.Address().Bytes()),
						).WithType(stdlib.EVMAddressBytesCadenceType),
					),
				)
				_, output, err := vm.Run(
					ctx,
					script,
					snapshot)
				require.NoError(t, err)
				require.NoError(t, output.Err)
			})
	})

	t.Run("testing calling Cadence arch - COA ownership proof (happy case)", func(t *testing.T) {
		chain := flow.Emulator.Chain()
		sc := systemcontracts.SystemContractsForChain(chain.ChainID())
		RunWithNewEnvironment(t,
			chain, func(
				ctx fvm.Context,
				vm fvm.VM,
				snapshot snapshot.SnapshotTree,
				testContract *TestContract,
				testAccount *EOATestAccount,
			) {
				// create a flow account
				privateKey, err := testutil.GenerateAccountPrivateKey()
				require.NoError(t, err)

				snapshot, accounts, err := testutil.CreateAccounts(
					vm,
					snapshot,
					[]flow.AccountPrivateKey{privateKey},
					chain)
				require.NoError(t, err)
				account := accounts[0]

				// create a COA and store it under flow account
				script := []byte(fmt.Sprintf(
					`
					import EVM from %s

					transaction {
						prepare(account: AuthAccount) {
							let cadenceOwnedAccount1 <- EVM.createCadenceOwnedAccount()
							account.save<@EVM.CadenceOwnedAccount>(<-cadenceOwnedAccount1,
																to: /storage/coa)
							account.link<&EVM.CadenceOwnedAccount{EVM.Addressable}>(/public/coa,
																				target: /storage/coa)
						}
					}
                `,
					sc.EVMContract.Address.HexWithPrefix(),
				))

				tx := fvm.Transaction(
					flow.NewTransactionBody().
						SetScript(script).
						AddAuthorizer(account),
					0)
				es, output, err := vm.Run(ctx, tx, snapshot)
				require.NoError(t, err)
				require.NoError(t, output.Err)
				snapshot = snapshot.Append(es)

				// 3rd event is the cadence owned account created event
				coaAddress, err := types.COAAddressFromFlowEvent(sc.EVMContract.Address, output.Events[2])
				require.NoError(t, err)

				data := RandomCommonHash(t)

				hasher, err := crypto.NewPrefixedHashing(privateKey.HashAlgo, "FLOW-V0.0-user")
				require.NoError(t, err)

				sig, err := privateKey.PrivateKey.Sign(data.Bytes(), hasher)
				require.NoError(t, err)

				proof := types.COAOwnershipProof{
					KeyIndices:     []uint64{0},
					Address:        types.FlowAddress(account),
					CapabilityPath: "coa",
					Signatures:     []types.Signature{types.Signature(sig)},
				}

				encodedProof, err := proof.Encode()
				require.NoError(t, err)

				// create transaction for proof verification
				code := []byte(fmt.Sprintf(
					`
					import EVM from %s

					access(all)
					fun main(tx: [UInt8], coinbaseBytes: [UInt8; 20]) {
						let coinbase = EVM.EVMAddress(bytes: coinbaseBytes)
						EVM.run(tx: tx, coinbase: coinbase)
					}
                	`,
					sc.EVMContract.Address.HexWithPrefix(),
				))
				innerTxBytes := testAccount.PrepareSignAndEncodeTx(t,
					testContract.DeployedAt.ToCommon(),
					testContract.MakeCallData(t, "verifyArchCallToVerifyCOAOwnershipProof",
						true,
						coaAddress.ToCommon(),
						data,
						encodedProof),
					big.NewInt(0),
					uint64(10_000_000),
					big.NewInt(0),
				)
				verifyScript := fvm.Script(code).WithArguments(
					json.MustEncode(
						cadence.NewArray(
							ConvertToCadence(innerTxBytes),
						).WithType(
							stdlib.EVMTransactionBytesCadenceType,
						)),
					json.MustEncode(
						cadence.NewArray(
							ConvertToCadence(
								testAccount.Address().Bytes(),
							),
						).WithType(
							stdlib.EVMAddressBytesCadenceType,
						),
					),
				)
				// run proof transaction
				_, output, err = vm.Run(
					ctx,
					verifyScript,
					snapshot)
				require.NoError(t, err)
				require.NoError(t, output.Err)
			})
	})
}

func RunWithNewEnvironment(
	t *testing.T,
	chain flow.Chain,
	f func(
		fvm.Context,
		fvm.VM,
		snapshot.SnapshotTree,
		*TestContract,
		*EOATestAccount,
	),
) {
	rootAddr, err := evm.StorageAccountAddress(chain.ChainID())
	require.NoError(t, err)

	RunWithTestBackend(t, func(backend *TestBackend) {
		RunWithDeployedContract(t, GetStorageTestContract(t), backend, rootAddr, func(testContract *TestContract) {
			RunWithEOATestAccount(t, backend, rootAddr, func(testAccount *EOATestAccount) {

				blocks := new(envMock.Blocks)
				block1 := unittest.BlockFixture()
				blocks.On("ByHeightFrom",
					block1.Header.Height,
					block1.Header,
				).Return(block1.Header, nil)

				opts := []fvm.Option{
					fvm.WithChain(chain),
					fvm.WithBlockHeader(block1.Header),
					fvm.WithAuthorizationChecksEnabled(false),
					fvm.WithSequenceNumberCheckAndIncrementEnabled(false),
					fvm.WithEntropyProvider(testutil.EntropyProviderFixture(nil)),
				}
				ctx := fvm.NewContext(opts...)

				vm := fvm.NewVirtualMachine()
				snapshotTree := snapshot.NewSnapshotTree(backend)

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

				f(fvm.NewContextFromParent(ctx, fvm.WithEVMEnabled(true)), vm, snapshotTree, testContract, testAccount)
			})
		})
	})
}
