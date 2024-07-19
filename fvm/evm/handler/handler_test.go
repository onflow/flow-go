package handler_test

import (
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"testing"
	"time"

	"github.com/onflow/cadence/runtime/common"
	gethCommon "github.com/onflow/go-ethereum/common"
	gethCore "github.com/onflow/go-ethereum/core"
	gethTypes "github.com/onflow/go-ethereum/core/types"
	"github.com/onflow/go-ethereum/core/vm"
	gethVM "github.com/onflow/go-ethereum/core/vm"
	gethParams "github.com/onflow/go-ethereum/params"
	"github.com/onflow/go-ethereum/rlp"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/evm/debug"
	"github.com/onflow/flow-go/fvm/evm/emulator"
	"github.com/onflow/flow-go/fvm/evm/handler"
	"github.com/onflow/flow-go/fvm/evm/handler/coa"
	"github.com/onflow/flow-go/fvm/evm/precompiles"
	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

var flowTokenAddress = common.MustBytesToAddress(systemcontracts.SystemContractsForChain(flow.Emulator).FlowToken.Address.Bytes())
var randomBeaconAddress = systemcontracts.SystemContractsForChain(flow.Emulator).RandomBeaconHistory.Address

func TestHandler_TransactionRunOrPanic(t *testing.T) {
	t.Parallel()

	t.Run("test RunOrPanic run (happy case)", func(t *testing.T) {
		t.Parallel()

		testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
			testutils.RunWithTestFlowEVMRootAddress(t, backend, func(rootAddr flow.Address) {
				testutils.RunWithEOATestAccount(t, backend, rootAddr, func(eoa *testutils.EOATestAccount) {

					bs := handler.NewBlockStore(backend, rootAddr)

					aa := handler.NewAddressAllocator()

					result := &types.Result{
						ReturnedData: testutils.RandomData(t),
						GasConsumed:  testutils.RandomGas(1000),
						Logs: []*gethTypes.Log{
							testutils.GetRandomLogFixture(t),
							testutils.GetRandomLogFixture(t),
						},
					}

					em := &testutils.TestEmulator{
						RunTransactionFunc: func(tx *gethTypes.Transaction) (*types.Result, error) {
							return result, nil
						},
					}
					handler := handler.NewContractHandler(flow.Emulator, rootAddr, flowTokenAddress, rootAddr, bs, aa, backend, em, debug.NopTracer)

					coinbase := types.NewAddress(gethCommon.Address{})

					tx := eoa.PrepareSignAndEncodeTx(
						t,
						gethCommon.Address{},
						nil,
						nil,
						100_000,
						big.NewInt(1),
					)

					// calculate tx id to match it
					var evmTx gethTypes.Transaction
					err := evmTx.UnmarshalBinary(tx)
					require.NoError(t, err)
					result.TxHash = evmTx.Hash()

					// successfully run (no-panic)
					handler.RunOrPanic(tx, coinbase)

					// check event
					events := backend.Events()
					require.Len(t, events, 1)
					txEventPayload := testutils.TxEventToPayload(t, events[0], rootAddr)
					require.NoError(t, err)
					// check logs
					var logs []*gethTypes.Log
					err = rlp.DecodeBytes(txEventPayload.Logs, &logs)
					require.NoError(t, err)
					for i, l := range result.Logs {
						assert.Equal(t, l, logs[i])
					}

					// form block
					handler.CommitBlockProposal()

					// check block event
					events = backend.Events()
					require.Len(t, events, 2)
					blockEventPayload := testutils.BlockEventToPayload(t, events[1], rootAddr)
					// make sure the transaction id included in the block transaction list is the same as tx sumbmitted
					assert.Equal(
						t,
						types.TransactionHashes{txEventPayload.Hash}.RootHash(),
						blockEventPayload.TransactionHashRoot,
					)
				})
			})
		})
	})

	t.Run("test RunOrPanic (unhappy non-fatal cases)", func(t *testing.T) {
		t.Parallel()

		testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
			testutils.RunWithTestFlowEVMRootAddress(t, backend, func(rootAddr flow.Address) {
				testutils.RunWithEOATestAccount(t, backend, rootAddr, func(eoa *testutils.EOATestAccount) {
					bs := handler.NewBlockStore(backend, rootAddr)
					aa := handler.NewAddressAllocator()
					em := &testutils.TestEmulator{
						RunTransactionFunc: func(tx *gethTypes.Transaction) (*types.Result, error) {
							return &types.Result{
								ValidationError: fmt.Errorf("some sort of validation error"),
							}, nil
						},
					}
					handler := handler.NewContractHandler(flow.Testnet, rootAddr, flowTokenAddress, rootAddr, bs, aa, backend, em, debug.NopTracer)

					coinbase := types.NewAddress(gethCommon.Address{})

					// test RLP decoding (non fatal)
					assertPanic(t, isNotFatal, func() {
						// invalid RLP encoding
						invalidTx := "badencoding"
						handler.RunOrPanic([]byte(invalidTx), coinbase)
					})

					// test gas limit (non fatal)
					assertPanic(t, isNotFatal, func() {
						gasLimit := uint64(testutils.TestComputationLimit + 1)
						tx := eoa.PrepareSignAndEncodeTx(
							t,
							gethCommon.Address{},
							nil,
							nil,
							gasLimit,
							big.NewInt(1),
						)

						handler.RunOrPanic([]byte(tx), coinbase)
					})

					// tx validation error
					assertPanic(t, isNotFatal, func() {
						// tx execution failure
						tx := eoa.PrepareSignAndEncodeTx(
							t,
							gethCommon.Address{},
							nil,
							nil,
							100_000,
							big.NewInt(1),
						)

						handler.RunOrPanic([]byte(tx), coinbase)
					})
				})
			})
		})

		t.Run("test RunOrPanic (fatal cases)", func(t *testing.T) {
			t.Parallel()

			testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
				testutils.RunWithTestFlowEVMRootAddress(t, backend, func(rootAddr flow.Address) {
					testutils.RunWithEOATestAccount(t, backend, rootAddr, func(eoa *testutils.EOATestAccount) {
						bs := handler.NewBlockStore(backend, rootAddr)
						aa := handler.NewAddressAllocator()
						em := &testutils.TestEmulator{
							RunTransactionFunc: func(tx *gethTypes.Transaction) (*types.Result, error) {
								return &types.Result{}, types.NewFatalError(fmt.Errorf("Fatal error"))
							},
						}
						handler := handler.NewContractHandler(flow.Testnet, rootAddr, flowTokenAddress, rootAddr, bs, aa, backend, em, debug.NopTracer)
						assertPanic(t, errors.IsFailure, func() {
							tx := eoa.PrepareSignAndEncodeTx(
								t,
								gethCommon.Address{},
								nil,
								nil,
								100_000,
								big.NewInt(1),
							)
							handler.RunOrPanic([]byte(tx), types.NewAddress(gethCommon.Address{}))
						})
					})
				})
			})
		})
	})

	t.Run("test RunOrPanic (with integrated emulator)", func(t *testing.T) {
		t.Parallel()

		testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
			testutils.RunWithTestFlowEVMRootAddress(t, backend, func(rootAddr flow.Address) {
				handler := SetupHandler(t, backend, rootAddr)

				eoa := testutils.GetTestEOAAccount(t, testutils.EOATestAccount1KeyHex)

				// deposit 1 Flow to the foa account
				addr := handler.DeployCOA(1)
				orgBalance := types.NewBalanceFromUFix64(types.OneFlowInUFix64)
				vault := types.NewFlowTokenVault(orgBalance)
				foa := handler.AccountByAddress(addr, true)
				foa.Deposit(vault)

				// transfer 0.1 flow to the non-foa address
				deduction := types.NewBalance(big.NewInt(1e17))
				foa.Call(eoa.Address(), nil, 400000, deduction)
				expected, err := types.SubBalance(orgBalance, deduction)
				require.NoError(t, err)
				require.Equal(t, expected, foa.Balance())

				// transfer 0.01 flow back to the foa through
				addition := types.NewBalance(big.NewInt(1e16))

				tx := eoa.PrepareSignAndEncodeTx(
					t,
					foa.Address().ToCommon(),
					nil,
					addition,
					gethParams.TxGas*10,
					big.NewInt(1e8), // high gas fee to test coinbase collection,
				)

				// setup coinbase
				foa2 := handler.DeployCOA(2)
				account2 := handler.AccountByAddress(foa2, true)
				require.True(t, types.BalancesAreEqual(
					types.NewBalanceFromUFix64(0),
					account2.Balance(),
				))

				// no panic means success here
				handler.RunOrPanic(tx, account2.Address())
				expected, err = types.SubBalance(orgBalance, deduction)
				require.NoError(t, err)
				expected, err = types.AddBalance(expected, addition)
				require.NoError(t, err)
				require.Equal(t, expected, foa.Balance())

				require.NotEqual(t, types.NewBalanceFromUFix64(0), account2.Balance())
			})
		})
	})
}

func TestHandler_OpsWithoutEmulator(t *testing.T) {
	t.Parallel()

	t.Run("test last executed block call", func(t *testing.T) {
		t.Parallel()

		testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
			testutils.RunWithTestFlowEVMRootAddress(t, backend, func(rootAddr flow.Address) {
				handler := SetupHandler(t, backend, rootAddr)

				// test call last executed block without initialization
				b := handler.LastExecutedBlock()
				require.Equal(t, types.GenesisBlock, b)

				// do some changes
				address := testutils.RandomAddress(t)
				account := handler.AccountByAddress(address, true)
				bal := types.OneFlowBalance
				account.Deposit(types.NewFlowTokenVault(bal))

				handler.CommitBlockProposal()
				// check if block height has been incremented
				b = handler.LastExecutedBlock()
				require.Equal(t, uint64(1), b.Height)
			})
		})
	})

	t.Run("test address allocation", func(t *testing.T) {
		t.Parallel()

		testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
			testutils.RunWithTestFlowEVMRootAddress(t, backend, func(rootAddr flow.Address) {
				h := SetupHandler(t, backend, rootAddr)

				coa := h.DeployCOA(12)
				require.NotNil(t, coa)

				expectedAddress := handler.MakeCOAAddress(12)
				require.Equal(t, expectedAddress, coa)
			})
		})
	})
}

func TestHandler_COA(t *testing.T) {
	t.Parallel()
	t.Run("test deposit/withdraw (with integrated emulator)", func(t *testing.T) {
		testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
			testutils.RunWithTestFlowEVMRootAddress(t, backend, func(rootAddr flow.Address) {
				handler := SetupHandler(t, backend, rootAddr)

				foa := handler.AccountByAddress(handler.DeployCOA(1), true)
				require.NotNil(t, foa)

				zeroBalance := types.NewBalance(big.NewInt(0))
				require.True(t, types.BalancesAreEqual(zeroBalance, foa.Balance()))

				balance := types.OneFlowBalance
				vault := types.NewFlowTokenVault(balance)

				foa.Deposit(vault)
				require.Equal(t, balance, foa.Balance())

				v := foa.Withdraw(balance)
				require.Equal(t, balance, v.Balance())

				require.True(t, types.BalancesAreEqual(
					zeroBalance, foa.Balance()))

				events := backend.Events()
				require.Len(t, events, 3)

				// Block level expected values
				txHashes := make(types.TransactionHashes, 0)
				totalGasUsed := uint64(0)

				// deploy COA transaction event
				txEventPayload := testutils.TxEventToPayload(t, events[0], rootAddr)
				tx, err := types.DirectCallFromEncoded(txEventPayload.Payload)
				require.NoError(t, err)
				txHashes = append(txHashes, tx.Hash())
				totalGasUsed += txEventPayload.GasConsumed

				// deposit transaction event
				txEventPayload = testutils.TxEventToPayload(t, events[1], rootAddr)
				tx, err = types.DirectCallFromEncoded(txEventPayload.Payload)
				require.NoError(t, err)
				require.Equal(t, foa.Address(), tx.To)
				require.Equal(t, types.BalanceToBigInt(balance), tx.Value)
				txHashes = append(txHashes, tx.Hash())
				totalGasUsed += txEventPayload.GasConsumed

				// withdraw transaction event
				txEventPayload = testutils.TxEventToPayload(t, events[2], rootAddr)
				tx, err = types.DirectCallFromEncoded(txEventPayload.Payload)
				require.NoError(t, err)
				require.Equal(t, foa.Address(), tx.From)
				require.Equal(t, types.BalanceToBigInt(balance), tx.Value)
				txHashes = append(txHashes, tx.Hash())
				totalGasUsed += txEventPayload.GasConsumed

				// block event
				handler.CommitBlockProposal()
				events = backend.Events()
				require.Len(t, events, 4)
				blockEventPayload := testutils.BlockEventToPayload(t, events[3], rootAddr)
				assert.Equal(
					t,
					txHashes.RootHash(),
					blockEventPayload.TransactionHashRoot,
				)

				require.Equal(t, totalGasUsed, blockEventPayload.TotalGasUsed)

				// check gas usage
				computationUsed, err := backend.ComputationUsed()
				require.NoError(t, err)
				require.Greater(t, computationUsed, types.DefaultDirectCallBaseGasUsage*3)

				// Withdraw with invalid balance
				assertPanic(t, types.IsWithdrawBalanceRoundingError, func() {
					// deposit some money
					foa.Deposit(vault)
					// then withdraw invalid balance
					foa.Withdraw(types.NewBalance(big.NewInt(1)))
				})
			})
		})
	})

	t.Run("test coa deployment", func(t *testing.T) {
		testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
			testutils.RunWithTestFlowEVMRootAddress(t, backend, func(rootAddr flow.Address) {
				h := SetupHandler(t, backend, rootAddr)

				coa1 := h.DeployCOA(1)
				acc := h.AccountByAddress(coa1, true)
				require.NotEmpty(t, acc.Code())

				// make a second account with some money
				coa2 := h.DeployCOA(2)
				acc2 := h.AccountByAddress(coa2, true)
				acc2.Deposit(types.NewFlowTokenVault(types.MakeABalanceInFlow(100)))

				// transfer money to COA
				acc2.Transfer(
					coa1,
					types.MakeABalanceInFlow(1),
				)

				// make a call to the contract
				ret := acc2.Call(
					coa1,
					testutils.MakeCallData(t,
						coa.ContractABIJSON,
						"onERC721Received",
						gethCommon.Address{1},
						gethCommon.Address{1},
						big.NewInt(0),
						[]byte{'A'},
					),
					types.GasLimit(3_000_000),
					types.EmptyBalance)

				// 0x150b7a02
				expected := types.Data([]byte{
					21, 11, 122, 2, 0, 0, 0, 0,
					0, 0, 0, 0, 0, 0, 0, 0,
					0, 0, 0, 0, 0, 0, 0, 0,
					0, 0, 0, 0, 0, 0, 0, 0,
				})
				require.Equal(t, types.StatusSuccessful, ret.Status)
				require.Equal(t, expected, ret.ReturnedData)
			})
		})
	})

	t.Run("test withdraw (unhappy case)", func(t *testing.T) {
		testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
			testutils.RunWithTestFlowEVMRootAddress(t, backend, func(rootAddr flow.Address) {
				testutils.RunWithEOATestAccount(t, backend, rootAddr, func(eoa *testutils.EOATestAccount) {
					bs := handler.NewBlockStore(backend, rootAddr)
					aa := handler.NewAddressAllocator()

					// Withdraw calls are only possible within FOA accounts
					assertPanic(t, types.IsAUnAuthroizedMethodCallError, func() {
						em := &testutils.TestEmulator{
							NonceOfFunc: func(address types.Address) (uint64, error) {
								return 0, nil
							},
						}

						handler := handler.NewContractHandler(flow.Testnet, rootAddr, flowTokenAddress, rootAddr, bs, aa, backend, em, debug.NopTracer)

						account := handler.AccountByAddress(testutils.RandomAddress(t), false)
						account.Withdraw(types.NewBalanceFromUFix64(1))
					})

					// test insufficient total supply error
					assertPanic(t, types.IsAInsufficientTotalSupplyError, func() {
						em := &testutils.TestEmulator{
							NonceOfFunc: func(address types.Address) (uint64, error) {
								return 0, nil
							},
							DirectCallFunc: func(call *types.DirectCall) (*types.Result, error) {
								return &types.Result{}, nil
							},
						}

						handler := handler.NewContractHandler(flow.Testnet, rootAddr, flowTokenAddress, rootAddr, bs, aa, backend, em, debug.NopTracer)
						account := handler.AccountByAddress(testutils.RandomAddress(t), true)

						account.Withdraw(types.NewBalanceFromUFix64(1))
					})

					// test non fatal error of emulator
					assertPanic(t, isNotFatal, func() {
						em := &testutils.TestEmulator{
							NonceOfFunc: func(address types.Address) (uint64, error) {
								return 0, nil
							},
							DirectCallFunc: func(call *types.DirectCall) (*types.Result, error) {
								return &types.Result{}, fmt.Errorf("some sort of error")
							},
						}

						handler := handler.NewContractHandler(flow.Testnet, rootAddr, flowTokenAddress, rootAddr, bs, aa, backend, em, debug.NopTracer)
						account := handler.AccountByAddress(testutils.RandomAddress(t), true)

						account.Withdraw(types.NewBalanceFromUFix64(0))
					})

					// test fatal error of emulator
					assertPanic(t, types.IsAFatalError, func() {
						em := &testutils.TestEmulator{
							NonceOfFunc: func(address types.Address) (uint64, error) {
								return 0, nil
							},
							DirectCallFunc: func(call *types.DirectCall) (*types.Result, error) {
								return &types.Result{}, types.NewFatalError(fmt.Errorf("some sort of fatal error"))
							},
						}

						handler := handler.NewContractHandler(flow.Testnet, rootAddr, flowTokenAddress, rootAddr, bs, aa, backend, em, debug.NopTracer)
						account := handler.AccountByAddress(testutils.RandomAddress(t), true)

						account.Withdraw(types.NewBalanceFromUFix64(0))
					})
				})
			})
		})
	})

	t.Run("test deposit (unhappy case)", func(t *testing.T) {
		t.Parallel()

		testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
			testutils.RunWithTestFlowEVMRootAddress(t, backend, func(rootAddr flow.Address) {
				testutils.RunWithEOATestAccount(t, backend, rootAddr, func(eoa *testutils.EOATestAccount) {
					bs := handler.NewBlockStore(backend, rootAddr)
					aa := handler.NewAddressAllocator()

					// test non fatal error of emulator
					assertPanic(t, isNotFatal, func() {
						em := &testutils.TestEmulator{
							NonceOfFunc: func(address types.Address) (uint64, error) {
								return 0, nil
							},
							DirectCallFunc: func(call *types.DirectCall) (*types.Result, error) {
								return &types.Result{}, fmt.Errorf("some sort of error")
							},
						}

						handler := handler.NewContractHandler(flow.Testnet, rootAddr, flowTokenAddress, rootAddr, bs, aa, backend, em, debug.NopTracer)
						account := handler.AccountByAddress(testutils.RandomAddress(t), true)

						account.Deposit(types.NewFlowTokenVault(types.NewBalanceFromUFix64(1)))
					})

					// test fatal error of emulator
					assertPanic(t, types.IsAFatalError, func() {
						em := &testutils.TestEmulator{
							NonceOfFunc: func(address types.Address) (uint64, error) {
								return 0, nil
							},
							DirectCallFunc: func(call *types.DirectCall) (*types.Result, error) {
								return &types.Result{}, types.NewFatalError(fmt.Errorf("some sort of fatal error"))
							},
						}

						handler := handler.NewContractHandler(flow.Testnet, rootAddr, flowTokenAddress, rootAddr, bs, aa, backend, em, debug.NopTracer)
						account := handler.AccountByAddress(testutils.RandomAddress(t), true)

						account.Deposit(types.NewFlowTokenVault(types.NewBalanceFromUFix64(1)))
					})
				})
			})
		})
	})

	t.Run("test deploy/call (with integrated emulator)", func(t *testing.T) {
		t.Parallel()

		// TODO update this test with events, gas metering, etc
		testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
			testutils.RunWithTestFlowEVMRootAddress(t, backend, func(rootAddr flow.Address) {
				handler := SetupHandler(t, backend, rootAddr)

				foa := handler.AccountByAddress(handler.DeployCOA(1), true)
				require.NotNil(t, foa)

				// deposit 10000 flow
				bal := types.MakeABalanceInFlow(10000)
				vault := types.NewFlowTokenVault(bal)
				foa.Deposit(vault)
				require.Equal(t, bal, foa.Balance())

				testContract := testutils.GetStorageTestContract(t)
				result := foa.Deploy(testContract.ByteCode, math.MaxUint64, types.NewBalanceFromUFix64(0))
				require.NotNil(t, result.DeployedContractAddress)
				addr := *result.DeployedContractAddress
				// skip first few bytes as they are deploy codes
				assert.Equal(t, testContract.ByteCode[17:], []byte(result.ReturnedData))
				require.NotNil(t, addr)

				num := big.NewInt(22)
				_ = foa.Call(
					addr,
					testContract.MakeCallData(t, "store", num),
					math.MaxUint64,
					types.NewBalanceFromUFix64(0))

				res := foa.Call(
					addr,
					testContract.MakeCallData(t, "retrieve"),
					math.MaxUint64,
					types.NewBalanceFromUFix64(0))

				require.Equal(t, num, res.ReturnedData.AsBigInt())
			})
		})
	})

	t.Run("test call to cadence arch", func(t *testing.T) {
		t.Parallel()

		testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
			blockHeight := uint64(123)
			backend.GetCurrentBlockHeightFunc = func() (uint64, error) {
				return blockHeight, nil
			}
			testutils.RunWithTestFlowEVMRootAddress(t, backend, func(rootAddr flow.Address) {
				h := SetupHandler(t, backend, rootAddr)

				foa := h.AccountByAddress(h.DeployCOA(1), true)
				require.NotNil(t, foa)

				vault := types.NewFlowTokenVault(types.MakeABalanceInFlow(10000))
				foa.Deposit(vault)

				arch := handler.MakePrecompileAddress(1)

				ret := foa.Call(arch, precompiles.FlowBlockHeightFuncSig[:], math.MaxUint64, types.NewBalanceFromUFix64(0))
				require.Equal(t, big.NewInt(int64(blockHeight)), new(big.Int).SetBytes(ret.ReturnedData))

				events := backend.Events()
				require.Len(t, events, 3)
				// last transaction executed event

				event := events[2]
				txEventPayload := testutils.TxEventToPayload(t, event, rootAddr)

				apc, err := types.AggregatedPrecompileCallsFromEncoded(txEventPayload.PrecompiledCalls)
				require.NoError(t, err)

				require.False(t, apc.IsEmpty())
				pc := apc[0]
				require.Equal(t, arch, pc.Address)
				require.Len(t, pc.RequiredGasCalls, 1)
				require.Equal(t,
					pc.RequiredGasCalls[0],
					types.RequiredGasCall{
						Input:  precompiles.FlowBlockHeightFuncSig[:],
						Output: precompiles.FlowBlockHeightFixedGas,
					},
				)
				require.Len(t, pc.RunCalls, 1)
				require.Equal(t,
					pc.RunCalls[0],
					types.RunCall{
						Input:    precompiles.FlowBlockHeightFuncSig[:],
						Output:   ret.ReturnedData,
						ErrorMsg: "",
					},
				)
			})
		})
	})

	t.Run("test block.random call (with integrated emulator)", func(t *testing.T) {
		t.Parallel()

		testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
			random := testutils.RandomCommonHash(t)
			backend.ReadRandomFunc = func(buffer []byte) error {
				copy(buffer, random.Bytes())
				return nil
			}
			testutils.RunWithTestFlowEVMRootAddress(t, backend, func(rootAddr flow.Address) {
				handler := SetupHandler(t, backend, rootAddr)

				foa := handler.AccountByAddress(handler.DeployCOA(1), true)
				require.NotNil(t, foa)

				vault := types.NewFlowTokenVault(types.MakeABalanceInFlow(100))
				foa.Deposit(vault)

				testContract := testutils.GetStorageTestContract(t)
				result := foa.Deploy(testContract.ByteCode, math.MaxUint64, types.EmptyBalance)
				require.NotNil(t, result.DeployedContractAddress)
				addr := *result.DeployedContractAddress
				require.Equal(t, types.StatusSuccessful, result.Status)
				require.Equal(t, types.ErrCodeNoError, result.ErrorCode)

				ret := foa.Call(
					addr,
					testContract.MakeCallData(t, "random"),
					math.MaxUint64,
					types.EmptyBalance)

				require.Equal(t, random.Bytes(), []byte(ret.ReturnedData))
			})
		})
	})

	// TODO add test with test emulator for unhappy cases (emulator)
}

func TestHandler_TransactionRun(t *testing.T) {
	t.Parallel()

	t.Run("test - transaction run (success)", func(t *testing.T) {
		t.Parallel()

		testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
			testutils.RunWithTestFlowEVMRootAddress(t, backend, func(rootAddr flow.Address) {
				testutils.RunWithEOATestAccount(t, backend, rootAddr, func(eoa *testutils.EOATestAccount) {

					bs := handler.NewBlockStore(backend, rootAddr)
					aa := handler.NewAddressAllocator()

					result := &types.Result{
						ReturnedData: testutils.RandomData(t),
						GasConsumed:  testutils.RandomGas(1000),
						Logs: []*gethTypes.Log{
							testutils.GetRandomLogFixture(t),
							testutils.GetRandomLogFixture(t),
						},
					}

					em := &testutils.TestEmulator{
						RunTransactionFunc: func(tx *gethTypes.Transaction) (*types.Result, error) {
							return result, nil
						},
					}
					handler := handler.NewContractHandler(flow.Testnet, rootAddr, flowTokenAddress, rootAddr, bs, aa, backend, em, debug.NopTracer)
					tx := eoa.PrepareSignAndEncodeTx(
						t,
						gethCommon.Address{},
						nil,
						nil,
						100_000,
						big.NewInt(1),
					)

					rs := handler.Run(tx, types.NewAddress(gethCommon.Address{}))
					require.Equal(t, types.StatusSuccessful, rs.Status)
					require.Equal(t, result.GasConsumed, rs.GasConsumed)
					require.Equal(t, types.ErrCodeNoError, rs.ErrorCode)

				})
			})
		})
	})

	t.Run("test - transaction run (failed)", func(t *testing.T) {
		t.Parallel()

		testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
			testutils.RunWithTestFlowEVMRootAddress(t, backend, func(rootAddr flow.Address) {
				testutils.RunWithEOATestAccount(t, backend, rootAddr, func(eoa *testutils.EOATestAccount) {

					bs := handler.NewBlockStore(backend, rootAddr)
					aa := handler.NewAddressAllocator()

					result := &types.Result{
						VMError:      gethVM.ErrOutOfGas,
						ReturnedData: testutils.RandomData(t),
						GasConsumed:  testutils.RandomGas(1000),
						Logs: []*gethTypes.Log{
							testutils.GetRandomLogFixture(t),
							testutils.GetRandomLogFixture(t),
						},
					}

					em := &testutils.TestEmulator{
						RunTransactionFunc: func(tx *gethTypes.Transaction) (*types.Result, error) {
							return result, nil
						},
					}
					handler := handler.NewContractHandler(flow.Testnet, rootAddr, flowTokenAddress, rootAddr, bs, aa, backend, em, debug.NopTracer)

					tx := eoa.PrepareSignAndEncodeTx(
						t,
						gethCommon.Address{},
						nil,
						nil,
						100_000,
						big.NewInt(1),
					)

					rs := handler.Run(tx, types.NewAddress(gethCommon.Address{}))
					require.Equal(t, types.StatusFailed, rs.Status)
					require.Equal(t, result.GasConsumed, rs.GasConsumed)
					require.Equal(t, types.ExecutionErrCodeOutOfGas, rs.ErrorCode)

				})
			})
		})
	})

	t.Run("test - transaction run (unhappy cases)", func(t *testing.T) {
		t.Parallel()

		testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
			testutils.RunWithTestFlowEVMRootAddress(t, backend, func(rootAddr flow.Address) {
				testutils.RunWithEOATestAccount(t, backend, rootAddr, func(eoa *testutils.EOATestAccount) {
					bs := handler.NewBlockStore(backend, rootAddr)
					aa := handler.NewAddressAllocator()
					evmErr := fmt.Errorf("%w: next nonce %v, tx nonce %v", gethCore.ErrNonceTooLow, 1, 0)
					em := &testutils.TestEmulator{
						RunTransactionFunc: func(tx *gethTypes.Transaction) (*types.Result, error) {
							return &types.Result{ValidationError: evmErr}, nil
						},
					}
					handler := handler.NewContractHandler(flow.Testnet, rootAddr, flowTokenAddress, rootAddr, bs, aa, backend, em, debug.NopTracer)

					coinbase := types.NewAddress(gethCommon.Address{})

					gasLimit := uint64(testutils.TestComputationLimit + 1)
					tx := eoa.PrepareSignAndEncodeTx(
						t,
						gethCommon.Address{},
						nil,
						nil,
						gasLimit,
						big.NewInt(1),
					)

					assertPanic(t, isNotFatal, func() {
						rs := handler.Run([]byte(tx), coinbase)
						require.Equal(t, types.StatusInvalid, rs.Status)
					})

					tx = eoa.PrepareSignAndEncodeTx(
						t,
						gethCommon.Address{},
						nil,
						nil,
						100,
						big.NewInt(1),
					)

					rs := handler.Run([]byte(tx), coinbase)
					require.Equal(t, types.StatusInvalid, rs.Status)
					require.Equal(t, types.ValidationErrCodeNonceTooLow, rs.ErrorCode)
				})
			})
		})
	})

	t.Run("test - transaction batch run (success)", func(t *testing.T) {
		t.Parallel()

		testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
			testutils.RunWithTestFlowEVMRootAddress(t, backend, func(rootAddr flow.Address) {
				testutils.RunWithEOATestAccount(t, backend, rootAddr, func(eoa *testutils.EOATestAccount) {
					bs := handler.NewBlockStore(backend, rootAddr)
					aa := handler.NewAddressAllocator()

					gasConsumed := testutils.RandomGas(1000)
					addr := testutils.RandomAddress(t)
					result := func(tx *gethTypes.Transaction) *types.Result {
						return &types.Result{
							DeployedContractAddress: &addr,
							ReturnedData:            testutils.RandomData(t),
							GasConsumed:             gasConsumed,
							TxHash:                  tx.Hash(),
							Logs: []*gethTypes.Log{
								testutils.GetRandomLogFixture(t),
								testutils.GetRandomLogFixture(t),
							},
						}
					}

					var runResults []*types.Result
					em := &testutils.TestEmulator{
						BatchRunTransactionFunc: func(txs []*gethTypes.Transaction) ([]*types.Result, error) {
							runResults = make([]*types.Result, len(txs))
							for i, tx := range txs {
								runResults[i] = result(tx)
							}
							return runResults, nil
						},
					}
					handler := handler.NewContractHandler(flow.Testnet, rootAddr, flowTokenAddress, randomBeaconAddress, bs, aa, backend, em, debug.NopTracer)

					coinbase := types.NewAddress(gethCommon.Address{})
					gasLimit := uint64(100_000)

					// run multiple successful transactions
					const batchSize = 3
					txs := make([][]byte, batchSize)
					for i := range txs {
						txs[i] = eoa.PrepareSignAndEncodeTx(
							t,
							gethCommon.Address{},
							nil,
							nil,
							gasLimit,
							big.NewInt(1),
						)
					}

					results := handler.BatchRun(txs, coinbase)
					for _, rs := range results {
						require.Equal(t, types.StatusSuccessful, rs.Status)
						require.Equal(t, gasConsumed, rs.GasConsumed)
						require.Equal(t, types.ErrCodeNoError, rs.ErrorCode)
					}

					handler.CommitBlockProposal()
					events := backend.Events()
					require.Len(t, events, batchSize+1) // +1 block event

					for i, event := range events {
						if i == batchSize {
							continue // don't check last block event
						}
						txEventPayload := testutils.TxEventToPayload(t, event, rootAddr)

						var logs []*gethTypes.Log
						err := rlp.DecodeBytes(txEventPayload.Logs, &logs)
						require.NoError(t, err)

						for k, l := range runResults[i].Logs {
							assert.Equal(t, l, logs[k])
						}
					}

					// run single transaction passed in as batch
					txs = make([][]byte, 1)
					for i := range txs {
						txs[i] = eoa.PrepareSignAndEncodeTx(
							t,
							gethCommon.Address{},
							nil,
							nil,
							gasLimit,
							big.NewInt(1),
						)
					}

					results = handler.BatchRun(txs, coinbase)
					for _, rs := range results {
						require.Equal(t, types.StatusSuccessful, rs.Status)
					}
				})
			})
		})
	})

	t.Run("test - transaction batch run (unhappy case)", func(t *testing.T) {
		t.Parallel()

		testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
			testutils.RunWithTestFlowEVMRootAddress(t, backend, func(rootAddr flow.Address) {
				testutils.RunWithEOATestAccount(t, backend, rootAddr, func(eoa *testutils.EOATestAccount) {
					bs := handler.NewBlockStore(backend, rootAddr)
					aa := handler.NewAddressAllocator()

					gasConsumed := testutils.RandomGas(1000)
					addr := testutils.RandomAddress(t)
					result := func() *types.Result {
						return &types.Result{
							DeployedContractAddress: &addr,
							ReturnedData:            testutils.RandomData(t),
							GasConsumed:             gasConsumed,
							Logs: []*gethTypes.Log{
								testutils.GetRandomLogFixture(t),
								testutils.GetRandomLogFixture(t),
							},
						}
					}

					em := &testutils.TestEmulator{
						BatchRunTransactionFunc: func(txs []*gethTypes.Transaction) ([]*types.Result, error) {
							res := make([]*types.Result, len(txs))
							for i := range res {
								res[i] = result()
							}
							return res, nil
						},
					}
					handler := handler.NewContractHandler(flow.Testnet, rootAddr, flowTokenAddress, randomBeaconAddress, bs, aa, backend, em, debug.NopTracer)
					coinbase := types.NewAddress(gethCommon.Address{})

					// batch run empty transactions
					txs := make([][]byte, 1)
					assertPanic(t, isNotFatal, func() {
						handler.BatchRun(txs, coinbase)
					})

				})
			})
		})
	})

	t.Run("test dry run successful", func(t *testing.T) {
		t.Parallel()

		testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
			testutils.RunWithTestFlowEVMRootAddress(t, backend, func(rootAddr flow.Address) {
				testutils.RunWithEOATestAccount(t, backend, rootAddr, func(eoa *testutils.EOATestAccount) {

					bs := handler.NewBlockStore(backend, rootAddr)
					aa := handler.NewAddressAllocator()

					nonce := uint64(1)
					to := gethCommon.Address{1, 2}
					amount := big.NewInt(13)
					gasLimit := uint64(1337)
					gasPrice := big.NewInt(2000)
					data := []byte{1, 5}
					from := types.Address{3, 4}

					tx := gethTypes.NewTransaction(
						nonce,
						to,
						amount,
						gasLimit,
						gasPrice,
						data,
					)
					rlpTx, err := tx.MarshalBinary()
					require.NoError(t, err)

					addr := testutils.RandomAddress(t)
					result := &types.Result{
						DeployedContractAddress: &addr,
						ReturnedData:            testutils.RandomData(t),
						GasConsumed:             testutils.RandomGas(1000),
						Logs: []*gethTypes.Log{
							testutils.GetRandomLogFixture(t),
							testutils.GetRandomLogFixture(t),
						},
					}

					called := false
					em := &testutils.TestEmulator{
						DryRunTransactionFunc: func(tx *gethTypes.Transaction, address gethCommon.Address) (*types.Result, error) {
							assert.Equal(t, nonce, tx.Nonce())
							assert.Equal(t, &to, tx.To())
							assert.Equal(t, gasLimit, tx.Gas())
							assert.Equal(t, gasPrice, tx.GasPrice())
							assert.Equal(t, data, tx.Data())
							assert.Equal(t, from.ToCommon(), address)
							called = true
							return result, nil
						},
					}

					handler := handler.NewContractHandler(flow.Testnet, rootAddr, flowTokenAddress, randomBeaconAddress, bs, aa, backend, em, debug.NopTracer)

					rs := handler.DryRun(rlpTx, from)
					require.Equal(t, types.StatusSuccessful, rs.Status)
					require.Equal(t, result.GasConsumed, rs.GasConsumed)
					require.Equal(t, types.ErrCodeNoError, rs.ErrorCode)
					require.True(t, called)
				})
			})
		})
	})

	t.Run("transaction run with tracing", func(t *testing.T) {
		t.Parallel()

		testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
			testutils.RunWithTestFlowEVMRootAddress(t, backend, func(rootAddr flow.Address) {
				testutils.RunWithEOATestAccount(t, backend, rootAddr, func(eoa *testutils.EOATestAccount) {

					var traceResult json.RawMessage
					txID := gethCommon.HexToHash("0x1")
					blockID := flow.Identifier{0x02}
					uploaded := make(chan struct{})

					result := &types.Result{
						TxHash:       txID,
						ReturnedData: testutils.RandomData(t),
						Logs: []*gethTypes.Log{
							testutils.GetRandomLogFixture(t),
						},
					}

					uploader := &testutils.MockUploader{
						UploadFunc: func(id string, message json.RawMessage) error {
							assert.Equal(t, traceResult, message)
							assert.Equal(t, debug.TraceID(txID, blockID), id)
							close(uploaded)
							return nil
						},
					}

					tracer, err := debug.NewEVMCallTracer(uploader, zerolog.Nop())
					require.NoError(t, err)
					tracer.WithBlockID(blockID)

					bs := handler.NewBlockStore(backend, rootAddr)
					aa := handler.NewAddressAllocator()

					em := &testutils.TestEmulator{
						RunTransactionFunc: func(tx *gethTypes.Transaction) (*types.Result, error) {
							tr := tracer.TxTracer()
							// mock some calls
							from := eoa.Address().ToCommon()
							tr.OnTxStart(nil, tx, from)
							tr.OnEnter(0, byte(vm.ADD), from, *tx.To(), tx.Data(), 20, big.NewInt(2))
							tr.OnExit(0, []byte{0x02}, 200, nil, false)
							tr.OnTxEnd(result.Receipt(0), nil)

							traceResult, err = tr.GetResult()
							require.NoError(t, err)
							return result, nil
						},
					}

					handler := handler.NewContractHandler(flow.Testnet, rootAddr, flowTokenAddress, rootAddr, bs, aa, backend, em, tracer)

					tx := eoa.PrepareSignAndEncodeTx(
						t,
						gethCommon.Address{},
						nil,
						nil,
						100_000,
						big.NewInt(1),
					)

					_ = handler.Run(tx, types.NewAddress(gethCommon.Address{}))

					assert.Eventuallyf(t, func() bool {
						<-uploaded
						return true
					}, time.Second, time.Millisecond*100, "upload not executed")
				})
			})
		})
	})

	// this test makes sure that even if tracing process fails it doesn't affect the execution flow
	// it also makes sure the upload is retried and then we panic
	t.Run("test - transaction run with tracing failure", func(t *testing.T) {
		t.Parallel()

		testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
			testutils.RunWithTestFlowEVMRootAddress(t, backend, func(rootAddr flow.Address) {
				testutils.RunWithEOATestAccount(t, backend, rootAddr, func(eoa *testutils.EOATestAccount) {

					uploaded := make(chan struct{})
					result := &types.Result{
						TxHash:       gethCommon.HexToHash("0x1"),
						ReturnedData: testutils.RandomData(t),
						Logs: []*gethTypes.Log{
							testutils.GetRandomLogFixture(t),
						},
					}

					uploader := &testutils.MockUploader{
						UploadFunc: func(id string, message json.RawMessage) error {
							close(uploaded)
							panic("total failure")
						},
					}

					tracer, err := debug.NewEVMCallTracer(uploader, zerolog.Nop())
					require.NoError(t, err)
					tracer.WithBlockID(flow.Identifier{0x1})

					bs := handler.NewBlockStore(backend, rootAddr)
					aa := handler.NewAddressAllocator()

					em := &testutils.TestEmulator{
						RunTransactionFunc: func(tx *gethTypes.Transaction) (*types.Result, error) {
							return result, nil
						},
					}

					handler := handler.NewContractHandler(flow.Testnet, rootAddr, flowTokenAddress, rootAddr, bs, aa, backend, em, tracer)

					tx := eoa.PrepareSignAndEncodeTx(
						t,
						gethCommon.Address{},
						nil,
						nil,
						100_000,
						big.NewInt(1),
					)

					res := handler.Run(tx, types.NewAddress(gethCommon.Address{}))

					assert.Eventuallyf(t, func() bool {
						<-uploaded
						return true
					}, time.Second*5, time.Millisecond*100, "upload not executed")

					require.Equal(t, types.StatusSuccessful, res.Status)
				})
			})
		})
	})

	t.Run("test - transaction batch run with tracing", func(t *testing.T) {
		t.Parallel()

		testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
			testutils.RunWithTestFlowEVMRootAddress(t, backend, func(rootAddr flow.Address) {
				testutils.RunWithEOATestAccount(t, backend, rootAddr, func(eoa *testutils.EOATestAccount) {
					bs := handler.NewBlockStore(backend, rootAddr)
					aa := handler.NewAddressAllocator()

					const batchSize = 3
					runResults := make([]*types.Result, batchSize)
					traceResults := make([]json.RawMessage, batchSize)
					uploaded := make(chan struct{}, batchSize)
					uploadedVals := make(map[string]json.RawMessage)
					blockID := flow.Identifier{0x02}

					uploader := &testutils.MockUploader{
						UploadFunc: func(id string, message json.RawMessage) error {
							uploadedVals[id] = message
							uploaded <- struct{}{}
							return nil
						},
					}

					tracer, err := debug.NewEVMCallTracer(uploader, zerolog.Nop())
					require.NoError(t, err)
					tracer.WithBlockID(blockID)

					em := &testutils.TestEmulator{
						BatchRunTransactionFunc: func(txs []*gethTypes.Transaction) ([]*types.Result, error) {
							runResults = make([]*types.Result, len(txs))
							for i, tx := range txs {
								tr := tracer.TxTracer()
								// TODO(Ramtin): figure out me
								// tr.CaptureTxStart(200)
								// tr.CaptureTxEnd(150)

								traceResults[i], _ = tr.GetResult()
								runResults[i] = &types.Result{
									TxHash: tx.Hash(),
									Logs: []*gethTypes.Log{
										testutils.GetRandomLogFixture(t),
									},
								}
							}
							return runResults, nil
						},
					}

					handler := handler.NewContractHandler(flow.Testnet, rootAddr, flowTokenAddress, randomBeaconAddress, bs, aa, backend, em, tracer)

					coinbase := types.NewAddress(gethCommon.Address{})

					txs := make([][]byte, batchSize)
					for i := range txs {
						txs[i] = eoa.PrepareSignAndEncodeTx(
							t,
							gethCommon.Address{},
							nil,
							nil,
							100_000,
							big.NewInt(1),
						)
					}

					_ = handler.BatchRun(txs, coinbase)

					for i := 0; i < batchSize; i++ {
						assert.Eventuallyf(t, func() bool {
							<-uploaded
							return true
						}, time.Second, time.Millisecond*100, "upload not executed")
					}

					for i, r := range runResults {
						id := debug.TraceID(r.TxHash, blockID)
						val, ok := uploadedVals[id]
						require.True(t, ok)
						require.Equal(t, traceResults[i], val)
					}
				})
			})
		})
	})

	t.Run("test - open tracing", func(t *testing.T) {
		t.Parallel()

		testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
			testutils.RunWithTestFlowEVMRootAddress(t, backend, func(rootAddr flow.Address) {
				testutils.RunWithEOATestAccount(t, backend, rootAddr, func(eoa *testutils.EOATestAccount) {

					tx := gethTypes.NewTransaction(
						uint64(1),
						gethCommon.Address{1, 2},
						big.NewInt(13),
						uint64(0),
						big.NewInt(1000),
						[]byte{},
					)

					rlpTx, err := tx.MarshalBinary()
					require.NoError(t, err)

					handler := SetupHandler(t, backend, rootAddr)

					backend.ExpectedSpan(t, trace.FVMEVMDryRun)
					handler.DryRun(rlpTx, types.EmptyAddress)

					backend.ExpectedSpan(t, trace.FVMEVMRun)
					handler.Run(rlpTx, types.EmptyAddress)

					backend.ExpectedSpan(t, trace.FVMEVMBatchRun)
					handler.BatchRun([][]byte{rlpTx}, types.EmptyAddress)

					backend.ExpectedSpan(t, trace.FVMEVMDeployCOA)
					coa := handler.DeployCOA(1)

					acc := handler.AccountByAddress(coa, true)

					backend.ExpectedSpan(t, trace.FVMEVMCall)
					acc.Call(types.EmptyAddress, nil, 1000, types.EmptyBalance)

					backend.ExpectedSpan(t, trace.FVMEVMDeposit)
					acc.Deposit(types.NewFlowTokenVault(types.EmptyBalance))

					backend.ExpectedSpan(t, trace.FVMEVMWithdraw)
					acc.Withdraw(types.EmptyBalance)

					backend.ExpectedSpan(t, trace.FVMEVMDeploy)
					acc.Deploy(nil, 1, types.EmptyBalance)
				})
			})
		})
	})
}

func TestHandler_Metrics(t *testing.T) {
	t.Parallel()

	testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
		testutils.RunWithTestFlowEVMRootAddress(t, backend, func(rootAddr flow.Address) {
			testutils.RunWithEOATestAccount(t, backend, rootAddr, func(eoa *testutils.EOATestAccount) {
				result := &types.Result{
					GasConsumed:             testutils.RandomGas(1000),
					DeployedContractAddress: &types.EmptyAddress,
				}
				em := &testutils.TestEmulator{
					RunTransactionFunc: func(tx *gethTypes.Transaction) (*types.Result, error) {
						return result, nil
					},
					BatchRunTransactionFunc: func(txs []*gethTypes.Transaction) ([]*types.Result, error) {
						return []*types.Result{result, result}, nil
					},
					DirectCallFunc: func(call *types.DirectCall) (*types.Result, error) {
						return result, nil
					},
					NonceOfFunc: func(address types.Address) (uint64, error) {
						return 1, nil
					},
				}
				handler := handler.NewContractHandler(
					flow.Emulator,
					rootAddr,
					flowTokenAddress,
					rootAddr,
					handler.NewBlockStore(backend, rootAddr),
					handler.NewAddressAllocator(),
					backend,
					em,
					debug.NopTracer)

				tx := eoa.PrepareSignAndEncodeTx(
					t,
					gethCommon.Address{},
					nil,
					nil,
					100_000,
					big.NewInt(1),
				)
				// run tx
				called := 0
				backend.EVMTransactionExecutedFunc = func(gas uint64, isDirect, failed bool) {
					require.Equal(t, result.GasConsumed, gas)
					require.False(t, isDirect)
					require.False(t, failed)
					called += 1
				}
				handler.Run(tx, types.EmptyAddress)
				require.Equal(t, 1, called)

				// batch run
				backend.EVMTransactionExecutedFunc = func(gas uint64, isDirect, failed bool) {
					require.Equal(t, result.GasConsumed, gas)
					require.False(t, isDirect)
					require.False(t, failed)
					called += 1
				}
				handler.BatchRun([][]byte{tx, tx}, types.EmptyAddress)
				require.Equal(t, 3, called)

				// Direct call
				backend.EVMTransactionExecutedFunc = func(gas uint64, isDirect, failed bool) {
					require.Equal(t, result.GasConsumed, gas)
					require.True(t, isDirect)
					require.False(t, failed)
					called += 1
				}
				coaCounter := 0
				backend.SetNumberOfDeployedCOAsFunc = func(count uint64) {
					coaCounter = int(count)
				}
				handler.DeployCOA(0)
				require.Equal(t, 4, called)
				require.Equal(t, 1, coaCounter)

				// form block
				backend.EVMBlockExecutedFunc = func(txCount int, gasUsed uint64, totalSupply uint64) {
					require.Equal(t, 4, txCount)
					require.Equal(t, result.GasConsumed*4, gasUsed)
					require.Equal(t, uint64(0), totalSupply)
					called += 1
				}
				handler.CommitBlockProposal()
				require.Equal(t, 5, called)
			})
		})
	})
}

// returns true if error passes the checks
type checkError func(error) bool

var isNotFatal = func(err error) bool {
	return !errors.IsFailure(err)
}

func assertPanic(t *testing.T, check checkError, f func()) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("The code did not panic")
		}
		err, ok := r.(error)
		if !ok {
			t.Fatal("panic is not with an error type")
		}
		require.True(t, check(err))
	}()
	f()
}

func SetupHandler(t testing.TB, backend types.Backend, rootAddr flow.Address) *handler.ContractHandler {
	bs := handler.NewBlockStore(backend, rootAddr)
	aa := handler.NewAddressAllocator()
	emulator := emulator.NewEmulator(backend, rootAddr)

	handler := handler.NewContractHandler(flow.Emulator, rootAddr, flowTokenAddress, rootAddr, bs, aa, backend, emulator, debug.NopTracer)
	return handler
}
