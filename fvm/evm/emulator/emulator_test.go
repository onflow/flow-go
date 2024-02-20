package emulator_test

import (
	"fmt"
	"math"
	"math/big"
	"testing"

	gethCommon "github.com/ethereum/go-ethereum/common"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	gethVM "github.com/ethereum/go-ethereum/core/vm"
	gethParams "github.com/ethereum/go-ethereum/params"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/evm/emulator"
	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

var blockNumber = big.NewInt(10)
var defaultCtx = types.NewDefaultBlockContext(blockNumber.Uint64())

func RunWithNewEmulator(t testing.TB, backend *testutils.TestBackend, rootAddr flow.Address, f func(*emulator.Emulator)) {
	env := emulator.NewEmulator(backend, rootAddr)
	f(env)
}

func RunWithNewBlockView(t testing.TB, em *emulator.Emulator, f func(blk types.BlockView)) {
	blk, err := em.NewBlockView(defaultCtx)
	require.NoError(t, err)
	f(blk)
}

func RunWithNewReadOnlyBlockView(t testing.TB, em *emulator.Emulator, f func(blk types.ReadOnlyBlockView)) {
	blk, err := em.NewReadOnlyBlockView(defaultCtx)
	require.NoError(t, err)
	f(blk)
}

func TestNativeTokenBridging(t *testing.T) {
	testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
		testutils.RunWithTestFlowEVMRootAddress(t, backend, func(rootAddr flow.Address) {
			originalBalance := big.NewInt(10000)
			testAccount := types.NewAddressFromString("test")
			nonce := uint64(0)

			t.Run("mint tokens to the first account", func(t *testing.T) {
				RunWithNewEmulator(t, backend, rootAddr, func(env *emulator.Emulator) {
					RunWithNewBlockView(t, env, func(blk types.BlockView) {
						res, err := blk.DirectCall(types.NewDepositCall(testAccount, originalBalance, nonce))
						require.NoError(t, err)
						require.Equal(t, defaultCtx.DirectCallBaseGasUsage, res.GasConsumed)
						nonce += 1
					})
				})
			})
			t.Run("tokens withdraw", func(t *testing.T) {
				amount := big.NewInt(1000)
				RunWithNewEmulator(t, backend, rootAddr, func(env *emulator.Emulator) {
					RunWithNewReadOnlyBlockView(t, env, func(blk types.ReadOnlyBlockView) {
						retBalance, err := blk.BalanceOf(testAccount)
						require.NoError(t, err)
						require.Equal(t, originalBalance, retBalance)
					})
				})
				RunWithNewEmulator(t, backend, rootAddr, func(env *emulator.Emulator) {
					RunWithNewBlockView(t, env, func(blk types.BlockView) {
						res, err := blk.DirectCall(types.NewWithdrawCall(testAccount, amount, nonce))
						require.NoError(t, err)
						require.Equal(t, defaultCtx.DirectCallBaseGasUsage, res.GasConsumed)
						nonce += 1
					})
				})
				RunWithNewEmulator(t, backend, rootAddr, func(env *emulator.Emulator) {
					RunWithNewReadOnlyBlockView(t, env, func(blk types.ReadOnlyBlockView) {
						retBalance, err := blk.BalanceOf(testAccount)
						require.NoError(t, err)
						require.Equal(t, amount.Sub(originalBalance, amount), retBalance)
					})
				})
			})
		})
	})
}

func TestContractInteraction(t *testing.T) {
	t.Parallel()
	testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
		testutils.RunWithTestFlowEVMRootAddress(t, backend, func(rootAddr flow.Address) {

			testContract := testutils.GetStorageTestContract(t)

			testAccount := types.NewAddressFromString("test")
			nonce := uint64(0)

			amount := big.NewInt(0).Mul(big.NewInt(1337), big.NewInt(gethParams.Ether))
			amountToBeTransfered := big.NewInt(0).Mul(big.NewInt(100), big.NewInt(gethParams.Ether))

			// fund test account
			RunWithNewEmulator(t, backend, rootAddr, func(env *emulator.Emulator) {
				RunWithNewBlockView(t, env, func(blk types.BlockView) {
					_, err := blk.DirectCall(types.NewDepositCall(testAccount, amount, nonce))
					require.NoError(t, err)
					nonce += 1
				})
			})

			var contractAddr types.Address

			t.Run("deploy contract", func(t *testing.T) {
				RunWithNewEmulator(t, backend, rootAddr, func(env *emulator.Emulator) {
					RunWithNewBlockView(t, env, func(blk types.BlockView) {
						res, err := blk.DirectCall(
							types.NewDeployCall(
								testAccount,
								testContract.ByteCode,
								math.MaxUint64,
								amountToBeTransfered,
								nonce),
						)
						require.NoError(t, err)
						contractAddr = res.DeployedContractAddress
						nonce += 1
					})
					RunWithNewReadOnlyBlockView(t, env, func(blk types.ReadOnlyBlockView) {
						require.NotNil(t, contractAddr)
						retCode, err := blk.CodeOf(contractAddr)
						require.NoError(t, err)
						require.NotEmpty(t, retCode)

						retBalance, err := blk.BalanceOf(contractAddr)
						require.NoError(t, err)
						require.Equal(t, amountToBeTransfered, retBalance)

						retBalance, err = blk.BalanceOf(testAccount)
						require.NoError(t, err)
						require.Equal(t, amount.Sub(amount, amountToBeTransfered), retBalance)
					})
				})
			})

			t.Run("call contract", func(t *testing.T) {
				num := big.NewInt(10)
				RunWithNewEmulator(t, backend, rootAddr, func(env *emulator.Emulator) {
					RunWithNewBlockView(t, env, func(blk types.BlockView) {
						res, err := blk.DirectCall(
							types.NewContractCall(
								testAccount,
								contractAddr,
								testContract.MakeCallData(t, "store", num),
								1_000_000,
								big.NewInt(0), // this should be zero because the contract doesn't have receiver
								nonce,
							),
						)
						require.NoError(t, err)
						require.GreaterOrEqual(t, res.GasConsumed, uint64(40_000))
						nonce += 1
					})
				})

				RunWithNewEmulator(t, backend, rootAddr, func(env *emulator.Emulator) {
					RunWithNewBlockView(t, env, func(blk types.BlockView) {
						res, err := blk.DirectCall(
							types.NewContractCall(
								testAccount,
								contractAddr,
								testContract.MakeCallData(t, "retrieve"),
								1_000_000,
								big.NewInt(0), // this should be zero because the contract doesn't have receiver
								nonce,
							),
						)
						require.NoError(t, err)
						nonce += 1

						ret := new(big.Int).SetBytes(res.ReturnedValue)
						require.Equal(t, num, ret)
						require.GreaterOrEqual(t, res.GasConsumed, uint64(23_000))
					})
				})

				RunWithNewEmulator(t, backend, rootAddr, func(env *emulator.Emulator) {
					RunWithNewBlockView(t, env, func(blk types.BlockView) {
						res, err := blk.DirectCall(
							types.NewContractCall(
								testAccount,
								contractAddr,
								testContract.MakeCallData(t, "blockNumber"),
								1_000_000,
								big.NewInt(0), // this should be zero because the contract doesn't have receiver
								nonce,
							),
						)
						require.NoError(t, err)
						nonce += 1

						ret := new(big.Int).SetBytes(res.ReturnedValue)
						require.Equal(t, blockNumber, ret)
					})
				})
			})

			t.Run("test sending transactions (happy case)", func(t *testing.T) {
				account := testutils.GetTestEOAAccount(t, testutils.EOATestAccount1KeyHex)
				fAddr := account.Address()
				RunWithNewEmulator(t, backend, rootAddr, func(env *emulator.Emulator) {
					RunWithNewBlockView(t, env, func(blk types.BlockView) {
						_, err := blk.DirectCall(types.NewDepositCall(fAddr, amount, account.Nonce()))
						require.NoError(t, err)
					})
				})

				RunWithNewEmulator(t, backend, rootAddr, func(env *emulator.Emulator) {
					ctx := types.NewDefaultBlockContext(blockNumber.Uint64())
					ctx.GasFeeCollector = types.NewAddressFromString("coinbase")
					coinbaseOrgBalance := gethCommon.Big1
					// small amount of money to create account
					RunWithNewBlockView(t, env, func(blk types.BlockView) {
						_, err := blk.DirectCall(types.NewDepositCall(ctx.GasFeeCollector, coinbaseOrgBalance, 0))
						require.NoError(t, err)
					})

					blk, err := env.NewBlockView(ctx)
					require.NoError(t, err)
					tx := account.PrepareAndSignTx(
						t,
						testAccount.ToCommon(), // to
						nil,                    // data
						big.NewInt(1000),       // amount
						gethParams.TxGas,       // gas limit
						gethCommon.Big1,        // gas fee

					)
					_, err = blk.RunTransaction(tx)
					require.NoError(t, err)

					// check the balance of coinbase
					RunWithNewReadOnlyBlockView(t, env, func(blk2 types.ReadOnlyBlockView) {
						bal, err := blk2.BalanceOf(ctx.GasFeeCollector)
						require.NoError(t, err)
						expected := gethParams.TxGas*gethCommon.Big1.Uint64() + gethCommon.Big1.Uint64()
						require.Equal(t, expected, bal.Uint64())

						nonce, err := blk2.NonceOf(fAddr)
						require.NoError(t, err)
						require.Equal(t, 1, int(nonce))
					})
				})
			})
			t.Run("test sending transactions (invalid nonce)", func(t *testing.T) {
				account := testutils.GetTestEOAAccount(t, testutils.EOATestAccount1KeyHex)
				fAddr := account.Address()
				RunWithNewEmulator(t, backend, rootAddr, func(env *emulator.Emulator) {
					RunWithNewBlockView(t, env, func(blk types.BlockView) {
						_, err := blk.DirectCall(types.NewDepositCall(fAddr, amount, account.Nonce()))
						require.NoError(t, err)
					})
				})

				RunWithNewEmulator(t, backend, rootAddr, func(env *emulator.Emulator) {
					ctx := types.NewDefaultBlockContext(blockNumber.Uint64())
					blk, err := env.NewBlockView(ctx)
					require.NoError(t, err)
					tx := account.SignTx(t,
						gethTypes.NewTransaction(
							100,                    // nonce
							testAccount.ToCommon(), // to
							big.NewInt(1000),       // amount
							gethParams.TxGas,       // gas limit
							gethCommon.Big1,        // gas fee
							nil,                    // data
						),
					)
					_, err = blk.RunTransaction(tx)
					require.Error(t, err)
					require.True(t, types.IsEVMValidationError(err))
				})
			})

			t.Run("test sending transactions (bad signature)", func(t *testing.T) {
				RunWithNewEmulator(t, backend, rootAddr, func(env *emulator.Emulator) {
					ctx := types.NewDefaultBlockContext(blockNumber.Uint64())
					blk, err := env.NewBlockView(ctx)
					require.NoError(t, err)
					tx := gethTypes.NewTx(&gethTypes.LegacyTx{
						Nonce:    0,
						GasPrice: gethCommon.Big1,
						Gas:      gethParams.TxGas, // gas limit
						To:       nil,              // to
						Value:    big.NewInt(1000), // amount
						Data:     nil,              // data
						V:        big.NewInt(1),
						R:        big.NewInt(2),
						S:        big.NewInt(3),
					})
					_, err = blk.RunTransaction(tx)
					require.Error(t, err)
					require.True(t, types.IsEVMValidationError(err))
				})
			})
		})
	})
}

func TestDeployAtFunctionality(t *testing.T) {
	testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
		testutils.RunWithTestFlowEVMRootAddress(t, backend, func(rootAddr flow.Address) {
			testContract := testutils.GetStorageTestContract(t)
			testAccount := types.NewAddressFromString("test")
			amount := big.NewInt(0).Mul(big.NewInt(1337), big.NewInt(gethParams.Ether))
			amountToBeTransfered := big.NewInt(0).Mul(big.NewInt(100), big.NewInt(gethParams.Ether))

			// fund test account
			RunWithNewEmulator(t, backend, rootAddr, func(env *emulator.Emulator) {
				RunWithNewBlockView(t, env, func(blk types.BlockView) {
					_, err := blk.DirectCall(types.NewDepositCall(testAccount, amount, 0))
					require.NoError(t, err)
				})
			})

			t.Run("deploy contract at target address", func(t *testing.T) {
				RunWithNewEmulator(t, backend, rootAddr, func(env *emulator.Emulator) {
					target := types.Address{1, 2, 3}
					RunWithNewBlockView(t, env, func(blk types.BlockView) {
						res, err := blk.DirectCall(
							types.NewDeployCallWithTargetAddress(
								testAccount,
								target,
								testContract.ByteCode,
								math.MaxUint64,
								amountToBeTransfered,
								0,
							),
						)
						require.NoError(t, err)
						require.Equal(t, target, res.DeployedContractAddress)
					})
					RunWithNewReadOnlyBlockView(t, env, func(blk types.ReadOnlyBlockView) {
						require.NotNil(t, target)
						retCode, err := blk.CodeOf(target)
						require.NoError(t, err)
						require.NotEmpty(t, retCode)

						retBalance, err := blk.BalanceOf(target)
						require.NoError(t, err)
						require.Equal(t, amountToBeTransfered, retBalance)

						retBalance, err = blk.BalanceOf(testAccount)
						require.NoError(t, err)
						require.Equal(t, amount.Sub(amount, amountToBeTransfered), retBalance)
					})
					// test deployment to an address that is already exist
					RunWithNewBlockView(t, env, func(blk types.BlockView) {
						res, err := blk.DirectCall(
							types.NewDeployCallWithTargetAddress(
								testAccount,
								target,
								testContract.ByteCode,
								math.MaxUint64,
								amountToBeTransfered,
								0),
						)
						require.NoError(t, err)
						require.Equal(t, gethVM.ErrContractAddressCollision, res.VMError)
					})
					// test deployment with not enough gas
					RunWithNewBlockView(t, env, func(blk types.BlockView) {
						res, err := blk.DirectCall(
							types.NewDeployCallWithTargetAddress(
								testAccount,
								types.Address{3, 4, 5},
								testContract.ByteCode,
								100,
								new(big.Int),
								0),
						)
						require.NoError(t, err)
						require.Equal(t, fmt.Errorf("out of gas"), res.VMError)
					})
				})
			})
		})
	})
}

func TestTransfers(t *testing.T) {
	testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
		testutils.RunWithTestFlowEVMRootAddress(t, backend, func(rootAddr flow.Address) {

			testAccount1 := types.NewAddressFromString("test1")
			testAccount2 := types.NewAddressFromString("test2")

			amount := big.NewInt(0).Mul(big.NewInt(1337), big.NewInt(gethParams.Ether))
			amountToBeTransfered := big.NewInt(0).Mul(big.NewInt(100), big.NewInt(gethParams.Ether))

			RunWithNewEmulator(t, backend, rootAddr, func(em *emulator.Emulator) {
				RunWithNewBlockView(t, em, func(blk types.BlockView) {
					_, err := blk.DirectCall(types.NewDepositCall(testAccount1, amount, 0))
					require.NoError(t, err)
				})
			})

			RunWithNewEmulator(t, backend, rootAddr, func(em *emulator.Emulator) {
				RunWithNewBlockView(t, em, func(blk types.BlockView) {
					_, err := blk.DirectCall(types.NewTransferCall(testAccount1, testAccount2, amountToBeTransfered, 0))
					require.NoError(t, err)
				})
			})

			RunWithNewEmulator(t, backend, rootAddr, func(em *emulator.Emulator) {
				RunWithNewReadOnlyBlockView(t, em, func(blk types.ReadOnlyBlockView) {
					bal, err := blk.BalanceOf(testAccount2)
					require.NoError(t, err)
					require.Equal(t, amountToBeTransfered.Uint64(), bal.Uint64())

					bal, err = blk.BalanceOf(testAccount1)
					require.NoError(t, err)
					require.Equal(t, new(big.Int).Sub(amount, amountToBeTransfered).Uint64(), bal.Uint64())
				})
			})
		})
	})
}

func TestStorageNoSideEffect(t *testing.T) {
	testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
		testutils.RunWithTestFlowEVMRootAddress(t, backend, func(flowEVMRoot flow.Address) {
			var err error
			em := emulator.NewEmulator(backend, flowEVMRoot)
			testAccount := types.NewAddressFromString("test")
			amount := big.NewInt(10)
			RunWithNewBlockView(t, em, func(blk types.BlockView) {
				_, err = blk.DirectCall(types.NewDepositCall(testAccount, amount, 0))
				require.NoError(t, err)
			})

			orgSize := backend.TotalStorageSize()
			RunWithNewBlockView(t, em, func(blk types.BlockView) {
				_, err = blk.DirectCall(types.NewDepositCall(testAccount, amount, 0))
				require.NoError(t, err)
			})
			require.Equal(t, orgSize, backend.TotalStorageSize())
		})
	})
}

func TestCallingExtraPrecompiles(t *testing.T) {
	testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
		testutils.RunWithTestFlowEVMRootAddress(t, backend, func(flowEVMRoot flow.Address) {
			RunWithNewEmulator(t, backend, flowEVMRoot, func(em *emulator.Emulator) {

				testAccount := types.NewAddressFromString("test")
				amount := big.NewInt(10_000_000)
				RunWithNewBlockView(t, em, func(blk types.BlockView) {
					_, err := blk.DirectCall(types.NewDepositCall(testAccount, amount, 0))
					require.NoError(t, err)
				})

				input := []byte{1, 2}
				output := []byte{3, 4}
				addr := testutils.RandomAddress(t)
				pc := &MockedPrecompile{
					AddressFunc: func() types.Address {
						return addr
					},
					RequiredGasFunc: func(input []byte) uint64 {
						return uint64(10)
					},
					RunFunc: func(inp []byte) ([]byte, error) {
						require.Equal(t, input, inp)
						return output, nil
					},
				}

				ctx := types.NewDefaultBlockContext(blockNumber.Uint64())
				ctx.ExtraPrecompiles = []types.Precompile{pc}

				blk, err := em.NewBlockView(ctx)
				require.NoError(t, err)

				res, err := blk.DirectCall(
					types.NewContractCall(
						testAccount,
						types.NewAddress(addr.ToCommon()),
						input,
						1_000_000,
						big.NewInt(0), // this should be zero because the contract doesn't have receiver
						0,
					),
				)
				require.NoError(t, err)
				require.Equal(t, output, res.ReturnedValue)
			})
		})
	})
}

type MockedPrecompile struct {
	AddressFunc     func() types.Address
	RequiredGasFunc func(input []byte) uint64
	RunFunc         func(input []byte) ([]byte, error)
}

func (mp *MockedPrecompile) Address() types.Address {
	if mp.AddressFunc == nil {
		panic("Address not set for the mocked precompile")
	}
	return mp.AddressFunc()
}

func (mp *MockedPrecompile) RequiredGas(input []byte) uint64 {
	if mp.RequiredGasFunc == nil {
		panic("RequiredGas not set for the mocked precompile")
	}
	return mp.RequiredGasFunc(input)
}

func (mp *MockedPrecompile) Run(input []byte) ([]byte, error) {
	if mp.RunFunc == nil {
		panic("Run not set for the mocked precompile")
	}
	return mp.RunFunc(input)
}
