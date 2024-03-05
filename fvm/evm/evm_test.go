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
	"github.com/onflow/flow-go/fvm/evm/testutils"
	. "github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestEVMRun(t *testing.T) {
	t.Parallel()

	chain := flow.Emulator.Chain()
	t.Run("testing EVM.run (happy case)", func(t *testing.T) {

		t.Parallel()
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

					transaction(tx: [UInt8], coinbaseBytes: [UInt8; 20]){
						prepare(account: AuthAccount) {
							let coinbase = EVM.EVMAddress(bytes: coinbaseBytes)
							let res = EVM.run(tx: tx, coinbase: coinbase)

							assert(res.status == EVM.Status.successful, message: "unexpected status")
							assert(res.errorCode == 0, message: "unexpected error code")
						}
					}
					`,
					sc.EVMContract.Address.HexWithPrefix(),
				))

				num := int64(12)
				innerTxBytes := testAccount.PrepareSignAndEncodeTx(t,
					testContract.DeployedAt.ToCommon(),
					testContract.MakeCallData(t, "store", big.NewInt(num)),
					big.NewInt(0),
					uint64(100_000),
					big.NewInt(0),
				)

				innerTx := cadence.NewArray(
					ConvertToCadence(innerTxBytes),
				).WithType(stdlib.EVMTransactionBytesCadenceType)

				coinbase := cadence.NewArray(
					ConvertToCadence(testAccount.Address().Bytes()),
				).WithType(stdlib.EVMAddressBytesCadenceType)

				tx := fvm.Transaction(
					flow.NewTransactionBody().
						SetScript(code).
						AddAuthorizer(sc.FlowServiceAccount.Address).
						AddArgument(json.MustEncode(innerTx)).
						AddArgument(json.MustEncode(coinbase)),
					0)

				state, output, err := vm.Run(
					ctx,
					tx,
					snapshot)
				require.NoError(t, err)
				require.NoError(t, output.Err)
				require.NotEmpty(t, state.WriteSet)

				// append the state
				snapshot = snapshot.Append(state)

				// query the value
				code = []byte(fmt.Sprintf(
					`
					import EVM from %s
					access(all)
					fun main(tx: [UInt8], coinbaseBytes: [UInt8; 20]): EVM.Result {
						let coinbase = EVM.EVMAddress(bytes: coinbaseBytes)
						return EVM.run(tx: tx, coinbase: coinbase)
					}
					`,
					sc.EVMContract.Address.HexWithPrefix(),
				))

				innerTxBytes = testAccount.PrepareSignAndEncodeTx(t,
					testContract.DeployedAt.ToCommon(),
					testContract.MakeCallData(t, "retrieve"),
					big.NewInt(0),
					uint64(100_000),
					big.NewInt(0),
				)

				innerTx = cadence.NewArray(
					ConvertToCadence(innerTxBytes),
				).WithType(stdlib.EVMTransactionBytesCadenceType)

				script := fvm.Script(code).WithArguments(
					json.MustEncode(innerTx),
					json.MustEncode(coinbase),
				)

				_, output, err = vm.Run(
					ctx,
					script,
					snapshot)
				require.NoError(t, err)
				require.NoError(t, output.Err)

				res, err := stdlib.ResultSummaryFromEVMResultValue(output.Value)
				require.NoError(t, err)
				require.Equal(t, types.StatusSuccessful, res.Status)
				require.Equal(t, types.ErrCodeNoError, res.ErrorCode)
				require.Equal(t, num, new(big.Int).SetBytes(res.ReturnedValue).Int64())
			})
	})

	t.Run("testing EVM.run (failed)", func(t *testing.T) {
		t.Parallel()
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

					transaction(tx: [UInt8], coinbaseBytes: [UInt8; 20]){
						prepare(account: AuthAccount) {
							let coinbase = EVM.EVMAddress(bytes: coinbaseBytes)
							let res = EVM.run(tx: tx, coinbase: coinbase)

							assert(res.status == EVM.Status.failed, message: "unexpected status")
							// ExecutionErrCodeExecutionReverted
							assert(res.errorCode == 306, message: "unexpected error code")
						}
					}
					`,
					sc.EVMContract.Address.HexWithPrefix(),
				))

				num := int64(12)
				innerTxBytes := testAccount.PrepareSignAndEncodeTx(t,
					testContract.DeployedAt.ToCommon(),
					testContract.MakeCallData(t, "storeButRevert", big.NewInt(num)),
					big.NewInt(0),
					uint64(100_000),
					big.NewInt(0),
				)

				innerTx := cadence.NewArray(
					ConvertToCadence(innerTxBytes),
				).WithType(stdlib.EVMTransactionBytesCadenceType)

				coinbase := cadence.NewArray(
					ConvertToCadence(testAccount.Address().Bytes()),
				).WithType(stdlib.EVMAddressBytesCadenceType)

				tx := fvm.Transaction(
					flow.NewTransactionBody().
						SetScript(code).
						AddAuthorizer(sc.FlowServiceAccount.Address).
						AddArgument(json.MustEncode(innerTx)).
						AddArgument(json.MustEncode(coinbase)),
					0)

				state, output, err := vm.Run(
					ctx,
					tx,
					snapshot)
				require.NoError(t, err)
				require.NoError(t, output.Err)
				require.NotEmpty(t, state.WriteSet)

				snapshot = snapshot.Append(state)

				// query the value
				code = []byte(fmt.Sprintf(
					`
					import EVM from %s
					access(all)
					fun main(tx: [UInt8], coinbaseBytes: [UInt8; 20]): EVM.Result {
						let coinbase = EVM.EVMAddress(bytes: coinbaseBytes)
						return EVM.run(tx: tx, coinbase: coinbase)
					}
				`,
					sc.EVMContract.Address.HexWithPrefix(),
				))

				innerTxBytes = testAccount.PrepareSignAndEncodeTx(t,
					testContract.DeployedAt.ToCommon(),
					testContract.MakeCallData(t, "retrieve"),
					big.NewInt(0),
					uint64(100_000),
					big.NewInt(0),
				)

				innerTx = cadence.NewArray(
					ConvertToCadence(innerTxBytes),
				).WithType(stdlib.EVMTransactionBytesCadenceType)

				script := fvm.Script(code).WithArguments(
					json.MustEncode(innerTx),
					json.MustEncode(coinbase),
				)

				_, output, err = vm.Run(
					ctx,
					script,
					snapshot)
				require.NoError(t, err)
				require.NoError(t, output.Err)

				res, err := stdlib.ResultSummaryFromEVMResultValue(output.Value)
				require.NoError(t, err)
				require.Equal(t, types.StatusSuccessful, res.Status)
				require.Equal(t, types.ErrCodeNoError, res.ErrorCode)
				require.Equal(t, int64(0), new(big.Int).SetBytes(res.ReturnedValue).Int64())
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

				transaction(addr: [UInt8; 20]) {
					prepare(account: AuthAccount) {
						let admin = account.borrow<&FlowToken.Administrator>(from: /storage/flowTokenAdmin)!
						let minter <- admin.createNewMinter(allowedAmount: 1.0)
						let vault <- minter.mintTokens(amount: 1.0)
						destroy minter

						let address = EVM.EVMAddress(addr)
						address.deposit(from: <-vault)
					}
				}
			`,
				sc.EVMContract.Address.HexWithPrefix(),
				sc.FlowToken.Address.HexWithPrefix(),
			))

			addr := RandomAddress(t)

			tx := fvm.Transaction(
				flow.NewTransactionBody().
					SetScript(code).
					AddAuthorizer(sc.FlowServiceAccount.Address).
					AddArgument(json.MustEncode(cadence.NewArray(
						ConvertToCadence(addr.Bytes()),
					).WithType(stdlib.EVMAddressBytesCadenceType))),
				0)

			execSnap, output, err := vm.Run(
				ctx,
				tx,
				snapshot)
			require.NoError(t, err)
			require.NoError(t, output.Err)

			snapshot = snapshot.Append(execSnap)

			expectedBalance := types.OneFlowBalance
			bal := getEVMAccountBalance(t, ctx, vm, snapshot, addr)
			require.Equal(t, expectedBalance, bal)
		})
}

func TestCOAAddressDeposit(t *testing.T) {
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

			_, output, err := vm.Run(
				ctx,
				script,
				snapshot)
			require.NoError(t, err)
			require.NoError(t, output.Err)

		})
}

func TestCadenceOwnedAccountFunctionalities(t *testing.T) {
	t.Parallel()
	chain := flow.Emulator.Chain()
	sc := systemcontracts.SystemContractsForChain(chain.ChainID())

	t.Run("test coa setup", func(t *testing.T) {
		t.Parallel()

		RunWithNewEnvironment(t,
			chain, func(
				ctx fvm.Context,
				vm fvm.VM,
				snapshot snapshot.SnapshotTree,
				testContract *TestContract,
				testAccount *EOATestAccount,
			) {
				// create a flow account
				flowAccount, _, snapshot := createAndFundFlowAccount(
					t,
					ctx,
					vm,
					snapshot,
				)

				var coaAddress types.Address

				initNonce := uint64(1)
				// 10 Flow in UFix64
				initBalanceInUFix64 := uint64(1_000_000_000)
				initBalance := types.NewBalanceFromUFix64(cadence.UFix64(initBalanceInUFix64))

				coaAddress, snapshot = setupCOA(
					t,
					ctx,
					vm,
					snapshot,
					flowAccount,
					initBalanceInUFix64)

				bal := getEVMAccountBalance(
					t,
					ctx,
					vm,
					snapshot,
					coaAddress)
				require.Equal(t, initBalance, bal)

				nonce := getEVMAccountNonce(
					t,
					ctx,
					vm,
					snapshot,
					coaAddress)
				require.Equal(t, initNonce, nonce)
			})
	})

	t.Run("test coa withdraw", func(t *testing.T) {
		t.Parallel()

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

				_, output, err := vm.Run(
					ctx,
					script,
					snapshot)
				require.NoError(t, err)
				require.NoError(t, output.Err)
			})
	})

	t.Run("test coa transfer", func(t *testing.T) {
		t.Parallel()

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
				fun main(address: [UInt8; 20]): UFix64 {
					let admin = getAuthAccount(%s)
						.borrow<&FlowToken.Administrator>(from: /storage/flowTokenAdmin)!
					let minter <- admin.createNewMinter(allowedAmount: 2.34)
					let vault <- minter.mintTokens(amount: 2.34)
					destroy minter

					let cadenceOwnedAccount <- EVM.createCadenceOwnedAccount()
					cadenceOwnedAccount.deposit(from: <-vault)

					let bal = EVM.Balance(0)
					bal.setFLOW(flow: 1.23)

					let recipientEVMAddress = EVM.EVMAddress(bytes: address)

					let res = cadenceOwnedAccount.call(
						to: recipientEVMAddress,
						data: [],
						gasLimit: 100_000,
						value: bal,
					)

					assert(res.status == EVM.Status.successful, message: "transfer call was not successful")

					destroy cadenceOwnedAccount
					return recipientEVMAddress.balance().inFLOW()
				}
				`,
					sc.EVMContract.Address.HexWithPrefix(),
					sc.FlowToken.Address.HexWithPrefix(),
					sc.FlowServiceAccount.Address.HexWithPrefix(),
				))

				addr := cadence.NewArray(
					ConvertToCadence(testutils.RandomAddress(t).Bytes()),
				).WithType(stdlib.EVMAddressBytesCadenceType)

				script := fvm.Script(code).WithArguments(
					json.MustEncode(addr),
				)

				_, output, err := vm.Run(
					ctx,
					script,
					snapshot)
				require.NoError(t, err)
				require.NoError(t, output.Err)

				require.Equal(t, uint64(123000000), uint64(output.Value.(cadence.UFix64)))
			})
	})

	t.Run("test coa deposit and withdraw in a single transaction", func(t *testing.T) {
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

				_, output, err := vm.Run(
					ctx,
					script,
					snapshot)
				require.NoError(t, err)
				require.NoError(t, output.Err)
			})
	})

	t.Run("test coa deploy", func(t *testing.T) {
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

				_, output, err := vm.Run(
					ctx,
					script,
					snapshot)
				require.NoError(t, err)
				require.NoError(t, output.Err)
			})
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
				flowAccount := accounts[0]

				// create/store/link coa
				coaAddress, snapshot := setupCOA(
					t,
					ctx,
					vm,
					snapshot,
					flowAccount,
					0,
				)

				data := RandomCommonHash(t)

				hasher, err := crypto.NewPrefixedHashing(privateKey.HashAlgo, "FLOW-V0.0-user")
				require.NoError(t, err)

				sig, err := privateKey.PrivateKey.Sign(data.Bytes(), hasher)
				require.NoError(t, err)

				proof := types.COAOwnershipProof{
					KeyIndices:     []uint64{0},
					Address:        types.FlowAddress(flowAccount),
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
				_, output, err := vm.Run(
					ctx,
					verifyScript,
					snapshot)
				require.NoError(t, err)
				require.NoError(t, output.Err)
			})
	})
}

func createAndFundFlowAccount(
	t *testing.T,
	ctx fvm.Context,
	vm fvm.VM,
	snapshot snapshot.SnapshotTree,
) (flow.Address, flow.AccountPrivateKey, snapshot.SnapshotTree) {

	privateKey, err := testutil.GenerateAccountPrivateKey()
	require.NoError(t, err)

	snapshot, accounts, err := testutil.CreateAccounts(
		vm,
		snapshot,
		[]flow.AccountPrivateKey{privateKey},
		ctx.Chain)
	require.NoError(t, err)
	flowAccount := accounts[0]

	// fund the account with 100 tokens
	sc := systemcontracts.SystemContractsForChain(ctx.Chain.ChainID())
	code := []byte(fmt.Sprintf(
		`
		import FlowToken from %s
		import FungibleToken from %s 

		transaction {
			prepare(account: AuthAccount) {
			let admin = account.borrow<&FlowToken.Administrator>(from: /storage/flowTokenAdmin)!
			let minter <- admin.createNewMinter(allowedAmount: 100.0)
			let vault <- minter.mintTokens(amount: 100.0)

			let receiverRef = getAccount(%s).getCapability(/public/flowTokenReceiver)
				.borrow<&{FungibleToken.Receiver}>()
				?? panic("Could not borrow receiver reference to the recipient's Vault")
			receiverRef.deposit(from: <-vault)

			destroy minter
			}
		}
		`,
		sc.FlowToken.Address.HexWithPrefix(),
		sc.FungibleToken.Address.HexWithPrefix(),
		flowAccount.HexWithPrefix(),
	))

	tx := fvm.Transaction(
		flow.NewTransactionBody().
			SetScript(code).
			AddAuthorizer(sc.FlowServiceAccount.Address),
		0)

	es, output, err := vm.Run(ctx, tx, snapshot)
	require.NoError(t, err)
	require.NoError(t, output.Err)
	snapshot = snapshot.Append(es)

	bal := getFlowAccountBalance(
		t,
		ctx,
		vm,
		snapshot,
		flowAccount)
	// 100 flow in ufix64
	require.Equal(t, uint64(10_000_000_000), bal)

	return flowAccount, privateKey, snapshot
}

func setupCOA(
	t *testing.T,
	ctx fvm.Context,
	vm fvm.VM,
	snap snapshot.SnapshotTree,
	coaOwner flow.Address,
	initialFund uint64,
) (types.Address, snapshot.SnapshotTree) {

	sc := systemcontracts.SystemContractsForChain(ctx.Chain.ChainID())
	// create a COA and store it under flow account
	script := []byte(fmt.Sprintf(
		`
	import EVM from %s
	import FungibleToken from %s
	import FlowToken from %s

	transaction(amount: UFix64) {
		prepare(account: AuthAccount) {
			let cadenceOwnedAccount1 <- EVM.createCadenceOwnedAccount()
			
			let vaultRef = account.borrow<&FlowToken.Vault>(from: /storage/flowTokenVault)
			?? panic("Could not borrow reference to the owner's Vault!")

			let vault <- vaultRef.withdraw(amount: amount) as! @FlowToken.Vault
			cadenceOwnedAccount1.deposit(from: <-vault)
			
			account.save<@EVM.CadenceOwnedAccount>(<-cadenceOwnedAccount1,
												to: /storage/coa)
			account.link<&EVM.CadenceOwnedAccount{EVM.Addressable}>(/public/coa,
																target: /storage/coa)
		}
	}
	`,
		sc.EVMContract.Address.HexWithPrefix(),
		sc.FungibleToken.Address.HexWithPrefix(),
		sc.FlowToken.Address.HexWithPrefix(),
	))

	tx := fvm.Transaction(
		flow.NewTransactionBody().
			SetScript(script).
			AddAuthorizer(coaOwner).
			AddArgument(json.MustEncode(cadence.UFix64(initialFund))),
		0)
	es, output, err := vm.Run(ctx, tx, snap)
	require.NoError(t, err)
	require.NoError(t, output.Err)
	snap = snap.Append(es)

	// 3rd event is the cadence owned account created event
	coaAddress, err := types.COAAddressFromFlowEvent(sc.EVMContract.Address, output.Events[2])
	require.NoError(t, err)

	return coaAddress, snap
}

func getFlowAccountBalance(
	t *testing.T,
	ctx fvm.Context,
	vm fvm.VM,
	snap snapshot.SnapshotTree,
	address flow.Address,
) uint64 {
	code := []byte(fmt.Sprintf(
		`
		pub fun main(): UFix64 {
			return getAccount(%s).balance
		}
		`,
		address.HexWithPrefix(),
	))

	script := fvm.Script(code)
	_, output, err := vm.Run(
		ctx,
		script,
		snap)
	require.NoError(t, err)
	require.NoError(t, output.Err)
	val, ok := output.Value.(cadence.UFix64)
	require.True(t, ok)
	return uint64(val)
}

func getEVMAccountBalance(
	t *testing.T,
	ctx fvm.Context,
	vm fvm.VM,
	snap snapshot.SnapshotTree,
	address types.Address,
) types.Balance {
	code := []byte(fmt.Sprintf(
		`
		import EVM from %s
		access(all)
		fun main(addr: [UInt8; 20]): UInt {
			return EVM.EVMAddress(bytes: addr).balance().inAttoFLOW()
		}
		`,
		systemcontracts.SystemContractsForChain(
			ctx.Chain.ChainID(),
		).EVMContract.Address.HexWithPrefix(),
	))

	script := fvm.Script(code).WithArguments(
		json.MustEncode(
			cadence.NewArray(
				ConvertToCadence(address.Bytes()),
			).WithType(stdlib.EVMAddressBytesCadenceType),
		),
	)
	_, output, err := vm.Run(
		ctx,
		script,
		snap)
	require.NoError(t, err)
	require.NoError(t, output.Err)
	val, ok := output.Value.(cadence.UInt)
	require.True(t, ok)
	return val.Big()
}

func getEVMAccountNonce(
	t *testing.T,
	ctx fvm.Context,
	vm fvm.VM,
	snap snapshot.SnapshotTree,
	address types.Address,
) uint64 {
	code := []byte(fmt.Sprintf(
		`
		import EVM from %s
		access(all)
		fun main(addr: [UInt8; 20]): UInt64 {
			return EVM.EVMAddress(bytes: addr).nonce()
		}
		`,
		systemcontracts.SystemContractsForChain(
			ctx.Chain.ChainID(),
		).EVMContract.Address.HexWithPrefix(),
	))

	script := fvm.Script(code).WithArguments(
		json.MustEncode(
			cadence.NewArray(
				ConvertToCadence(address.Bytes()),
			).WithType(stdlib.EVMAddressBytesCadenceType),
		),
	)
	_, output, err := vm.Run(
		ctx,
		script,
		snap)
	require.NoError(t, err)
	require.NoError(t, output.Err)
	val, ok := output.Value.(cadence.UInt64)
	require.True(t, ok)
	return uint64(val)
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
