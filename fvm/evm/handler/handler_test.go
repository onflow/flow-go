package handler_test

import (
	"encoding/hex"
	"fmt"
	"math"
	"math/big"
	"strings"
	"testing"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/runtime/common"
	gethCommon "github.com/onflow/go-ethereum/common"
	gethCore "github.com/onflow/go-ethereum/core"
	gethTypes "github.com/onflow/go-ethereum/core/types"
	gethVM "github.com/onflow/go-ethereum/core/vm"
	gethParams "github.com/onflow/go-ethereum/params"
	"github.com/onflow/go-ethereum/rlp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/evm/emulator"
	"github.com/onflow/flow-go/fvm/evm/handler"
	"github.com/onflow/flow-go/fvm/evm/handler/coa"
	"github.com/onflow/flow-go/fvm/evm/precompiles"
	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
)

// TODO add test for fatal errors

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
						ReturnedValue: testutils.RandomData(t),
						GasConsumed:   testutils.RandomGas(1000),
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
					handler := handler.NewContractHandler(flow.Emulator, rootAddr, flowTokenAddress, rootAddr, bs, aa, backend, em)

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

					// check gas usage
					// TODO: uncomment and investigate me
					// computationUsed, err := backend.ComputationUsed()
					// require.NoError(t, err)
					// require.Equal(t, result.GasConsumed, computationUsed)

					// check events (1 extra for block event)
					events := backend.Events()

					require.Len(t, events, 2)

					event := events[0]
					assert.Equal(t, event.Type, types.EventTypeTransactionExecuted)
					ev, err := jsoncdc.Decode(nil, event.Payload)
					require.NoError(t, err)
					cadenceEvent, ok := ev.(cadence.Event)
					require.True(t, ok)
					for j, f := range cadenceEvent.GetFields() {
						// todo add an event decoder in types.event
						if f.Identifier == "logs" {
							cadenceLogs := cadenceEvent.GetFieldValues()[j]
							encodedLogs, err := hex.DecodeString(strings.ReplaceAll(cadenceLogs.String(), "\"", ""))
							require.NoError(t, err)

							var logs []*gethTypes.Log
							err = rlp.DecodeBytes(encodedLogs, &logs)
							require.NoError(t, err)

							for i, l := range result.Logs {
								assert.Equal(t, l, logs[i])
							}
						}
					}

					// check block event
					event = events[1]

					assert.Equal(t, event.Type, types.EventTypeBlockExecuted)
					ev, err = jsoncdc.Decode(nil, event.Payload)
					require.NoError(t, err)

					// make sure block transaction list references the above transaction id
					cadenceEvent, ok = ev.(cadence.Event)
					require.True(t, ok)
					blockEvent, err := types.DecodeBlockEventPayload(cadenceEvent)
					require.NoError(t, err)

					eventTxID := blockEvent.TransactionHashes[0] // only one hash in block
					// make sure the transaction id included in the block transaction list is the same as tx sumbmitted
					assert.Equal(t, evmTx.Hash().String(), eventTxID.ToGoValue())
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
					handler := handler.NewContractHandler(flow.Testnet, rootAddr, flowTokenAddress, rootAddr, bs, aa, backend, em)

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
						handler := handler.NewContractHandler(flow.Testnet, rootAddr, flowTokenAddress, rootAddr, bs, aa, backend, em)
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
				require.Equal(t, types.NewBalanceFromUFix64(0), account2.Balance())

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
				require.Equal(t, zeroBalance, foa.Balance())

				balance := types.OneFlowBalance
				vault := types.NewFlowTokenVault(balance)

				foa.Deposit(vault)
				require.Equal(t, balance, foa.Balance())

				v := foa.Withdraw(balance)
				require.Equal(t, balance, v.Balance())

				require.Equal(t, zeroBalance, foa.Balance())

				events := backend.Events()
				require.Len(t, events, 6)

				// first two transactions are for COA setup

				// transaction event
				event := events[2]
				assert.Equal(t, event.Type, types.EventTypeTransactionExecuted)

				// block event
				event = events[3]
				assert.Equal(t, event.Type, types.EventTypeBlockExecuted)

				// transaction event
				event = events[4]
				assert.Equal(t, event.Type, types.EventTypeTransactionExecuted)
				_, err := jsoncdc.Decode(nil, event.Payload)
				require.NoError(t, err)
				// TODO: decode encoded tx and check for the amount and value
				// assert.Equal(t, foa.Address(), ret.Address)
				// assert.Equal(t, balance, ret.Amount)

				// block event
				event = events[5]
				assert.Equal(t, event.Type, types.EventTypeBlockExecuted)

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
				require.Equal(t, expected, ret.ReturnedValue)
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

						handler := handler.NewContractHandler(flow.Testnet, rootAddr, flowTokenAddress, rootAddr, bs, aa, backend, em)

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

						handler := handler.NewContractHandler(flow.Testnet, rootAddr, flowTokenAddress, rootAddr, bs, aa, backend, em)
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

						handler := handler.NewContractHandler(flow.Testnet, rootAddr, flowTokenAddress, rootAddr, bs, aa, backend, em)
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

						handler := handler.NewContractHandler(flow.Testnet, rootAddr, flowTokenAddress, rootAddr, bs, aa, backend, em)
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

						handler := handler.NewContractHandler(flow.Testnet, rootAddr, flowTokenAddress, rootAddr, bs, aa, backend, em)
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

						handler := handler.NewContractHandler(flow.Testnet, rootAddr, flowTokenAddress, rootAddr, bs, aa, backend, em)
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
				assert.Equal(t, testContract.ByteCode[17:], []byte(result.ReturnedValue))
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

				require.Equal(t, num, res.ReturnedValue.AsBigInt())
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
				require.Equal(t, big.NewInt(int64(blockHeight)), new(big.Int).SetBytes(ret.ReturnedValue))
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

				require.Equal(t, random.Bytes(), []byte(ret.ReturnedValue))
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
						ReturnedValue: testutils.RandomData(t),
						GasConsumed:   testutils.RandomGas(1000),
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
					handler := handler.NewContractHandler(flow.Testnet, rootAddr, flowTokenAddress, rootAddr, bs, aa, backend, em)
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
						VMError:       gethVM.ErrOutOfGas,
						ReturnedValue: testutils.RandomData(t),
						GasConsumed:   testutils.RandomGas(1000),
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
					handler := handler.NewContractHandler(flow.Testnet, rootAddr, flowTokenAddress, rootAddr, bs, aa, backend, em)

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
					handler := handler.NewContractHandler(flow.Testnet, rootAddr, flowTokenAddress, rootAddr, bs, aa, backend, em)

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
							ReturnedValue:           testutils.RandomData(t),
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
					handler := handler.NewContractHandler(flow.Testnet, rootAddr, flowTokenAddress, randomBeaconAddress, bs, aa, backend, em)

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

					events := backend.Events()
					require.Len(t, events, batchSize+1) // +1 block event

					for i, event := range events {
						if i == batchSize {
							continue // don't check last block event
						}
						assert.Equal(t, event.Type, types.EventTypeTransactionExecuted)
						ev, err := jsoncdc.Decode(nil, event.Payload)
						require.NoError(t, err)
						cadenceEvent, ok := ev.(cadence.Event)
						require.True(t, ok)
						for j, f := range cadenceEvent.GetFields() {
							if f.Identifier == "logs" {
								cadenceLogs := cadenceEvent.GetFieldValues()[j]
								encodedLogs, err := hex.DecodeString(strings.ReplaceAll(cadenceLogs.String(), "\"", ""))
								require.NoError(t, err)

								var logs []*gethTypes.Log
								err = rlp.DecodeBytes(encodedLogs, &logs)
								require.NoError(t, err)

								for k, l := range runResults[i].Logs {
									assert.Equal(t, l, logs[k])
								}
							}
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
							ReturnedValue:           testutils.RandomData(t),
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
					handler := handler.NewContractHandler(flow.Testnet, rootAddr, flowTokenAddress, randomBeaconAddress, bs, aa, backend, em)
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
						ReturnedValue:           testutils.RandomData(t),
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

					handler := handler.NewContractHandler(flow.Testnet, rootAddr, flowTokenAddress, randomBeaconAddress, bs, aa, backend, em)

					rs := handler.DryRun(rlpTx, from)
					require.Equal(t, types.StatusSuccessful, rs.Status)
					require.Equal(t, result.GasConsumed, rs.GasConsumed)
					require.Equal(t, types.ErrCodeNoError, rs.ErrorCode)
					require.True(t, called)
				})
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

	handler := handler.NewContractHandler(flow.Emulator, rootAddr, flowTokenAddress, rootAddr, bs, aa, backend, emulator)
	return handler
}
