package handler_test

import (
	"bytes"
	"fmt"
	"math"
	"math/big"
	"testing"

	gethCommon "github.com/ethereum/go-ethereum/common"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	gethParams "github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/evm/emulator"
	"github.com/onflow/flow-go/fvm/evm/emulator/database"
	"github.com/onflow/flow-go/fvm/evm/handler"
	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
)

// TODO add test for fatal errors

var flowTokenAddress = common.MustBytesToAddress(systemcontracts.SystemContractsForChain(flow.Emulator).FlowToken.Address.Bytes())

func TestHandler_TransactionRun(t *testing.T) {
	t.Parallel()

	t.Run("test - transaction run (happy case)", func(t *testing.T) {
		t.Parallel()

		testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
			testutils.RunWithTestFlowEVMRootAddress(t, backend, func(rootAddr flow.Address) {
				testutils.RunWithEOATestAccount(t, backend, rootAddr, func(eoa *testutils.EOATestAccount) {

					bs, err := handler.NewBlockStore(backend, rootAddr)
					require.NoError(t, err)

					aa, err := handler.NewAddressAllocator(backend, rootAddr)
					require.NoError(t, err)

					result := &types.Result{
						StateRootHash:           testutils.RandomCommonHash(t),
						DeployedContractAddress: types.Address(testutils.RandomAddress(t)),
						ReturnedValue:           testutils.RandomData(t),
						GasConsumed:             testutils.RandomGas(1000),
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
					handler := handler.NewContractHandler(flowTokenAddress, bs, aa, backend, em)

					coinbase := types.NewAddress(gethCommon.Address{})

					tx := eoa.PrepareSignAndEncodeTx(
						t,
						gethCommon.Address{},
						nil,
						nil,
						100_000,
						big.NewInt(1),
					)

					// successfully run (no-panic)
					handler.Run(tx, coinbase)

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
					ev := types.TransactionExecutedPayload{}
					err = rlp.Decode(bytes.NewReader(event.Payload), &ev)
					require.NoError(t, err)
					for i, l := range result.Logs {
						assert.Equal(t, l, ev.Result.Logs[i])
					}

					// check block event
					event = events[1]
					assert.Equal(t, event.Type, types.EventTypeBlockExecuted)
					payload := types.BlockExecutedEventPayload{}
					err = rlp.Decode(bytes.NewReader(event.Payload), &payload)
					require.NoError(t, err)
				})
			})
		})
	})

	t.Run("test - transaction run (unhappy cases)", func(t *testing.T) {
		t.Parallel()

		testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
			testutils.RunWithTestFlowEVMRootAddress(t, backend, func(rootAddr flow.Address) {
				testutils.RunWithEOATestAccount(t, backend, rootAddr, func(eoa *testutils.EOATestAccount) {

					bs, err := handler.NewBlockStore(backend, rootAddr)
					require.NoError(t, err)

					aa, err := handler.NewAddressAllocator(backend, rootAddr)
					require.NoError(t, err)

					em := &testutils.TestEmulator{
						RunTransactionFunc: func(tx *gethTypes.Transaction) (*types.Result, error) {
							return &types.Result{}, types.NewEVMExecutionError(fmt.Errorf("some sort of error"))
						},
					}
					handler := handler.NewContractHandler(flowTokenAddress, bs, aa, backend, em)

					coinbase := types.NewAddress(gethCommon.Address{})

					// test RLP decoding (non fatal)
					assertPanic(t, isNotFatal, func() {
						// invalid RLP encoding
						invalidTx := "badencoding"
						handler.Run([]byte(invalidTx), coinbase)
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

						handler.Run([]byte(tx), coinbase)
					})

					// tx execution failure
					tx := eoa.PrepareSignAndEncodeTx(
						t,
						gethCommon.Address{},
						nil,
						nil,
						100_000,
						big.NewInt(1),
					)

					assertPanic(t, isNotFatal, func() {
						handler.Run([]byte(tx), coinbase)
					})
				})
			})
		})
	})

	t.Run("test running transaction (with integrated emulator)", func(t *testing.T) {
		t.Parallel()

		testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
			testutils.RunWithTestFlowEVMRootAddress(t, backend, func(rootAddr flow.Address) {
				_, handler := SetupHandler(t, backend, rootAddr)

				eoa := testutils.GetTestEOAAccount(t, testutils.EOATestAccount1KeyHex)

				// deposit 1 Flow to the foa account
				addr := handler.AllocateAddress()
				orgBalance, err := types.NewBalanceFromAttoFlow(types.OneFlowInAttoFlow)
				require.NoError(t, err)
				vault := types.NewFlowTokenVault(orgBalance)
				foa := handler.AccountByAddress(addr, true)
				foa.Deposit(vault)

				// transfer 0.1 flow to the non-foa address
				deduction, err := types.NewBalanceFromAttoFlow(big.NewInt(1e17))
				require.NoError(t, err)
				foa.Call(eoa.Address(), nil, 400000, deduction)
				require.Equal(t, orgBalance.Sub(deduction), foa.Balance())

				// transfer 0.01 flow back to the foa through
				addition, err := types.NewBalanceFromAttoFlow(big.NewInt(1e16))
				require.NoError(t, err)

				tx := eoa.PrepareSignAndEncodeTx(
					t,
					foa.Address().ToCommon(),
					nil,
					addition.ToAttoFlow(),
					gethParams.TxGas*10,
					big.NewInt(1e8), // high gas fee to test coinbase collection,
				)

				// setup coinbase
				foa2 := handler.AllocateAddress()
				account2 := handler.AccountByAddress(foa2, true)
				require.Equal(t, types.Balance(0), account2.Balance())

				// no panic means success here
				handler.Run(tx, account2.Address())
				require.Equal(t, orgBalance.Sub(deduction).Add(addition), foa.Balance())

				// fees has been collected to the coinbase
				require.NotEqual(t, types.Balance(0), account2.Balance())

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
				_, handler := SetupHandler(t, backend, rootAddr)

				// test call last executed block without initialization
				b := handler.LastExecutedBlock()
				require.Equal(t, types.GenesisBlock, b)

				// do some changes
				address := testutils.RandomAddress(t)
				account := handler.AccountByAddress(address, true)
				bal, err := types.NewBalanceFromAttoFlow(types.OneFlowInAttoFlow)
				require.NoError(t, err)
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
				blockchain, err := handler.NewBlockStore(backend, rootAddr)
				require.NoError(t, err)

				aa, err := handler.NewAddressAllocator(backend, rootAddr)
				require.NoError(t, err)

				handler := handler.NewContractHandler(flowTokenAddress, blockchain, aa, backend, nil)

				foa := handler.AllocateAddress()
				require.NotNil(t, foa)

				expectedAddress := types.NewAddress(gethCommon.HexToAddress("0x00000000000000000001"))
				require.Equal(t, expectedAddress, foa)
			})
		})
	})
}

func TestHandler_BridgedAccount(t *testing.T) {

	t.Run("test deposit/withdraw (with integrated emulator)", func(t *testing.T) {
		t.Parallel()

		testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
			testutils.RunWithTestFlowEVMRootAddress(t, backend, func(rootAddr flow.Address) {
				_, handler := SetupHandler(t, backend, rootAddr)

				foa := handler.AccountByAddress(handler.AllocateAddress(), true)
				require.NotNil(t, foa)

				zeroBalance, err := types.NewBalanceFromAttoFlow(big.NewInt(0))
				require.NoError(t, err)
				require.Equal(t, zeroBalance, foa.Balance())

				balance, err := types.NewBalanceFromAttoFlow(types.OneFlowInAttoFlow)
				require.NoError(t, err)
				vault := types.NewFlowTokenVault(balance)

				foa.Deposit(vault)
				require.NoError(t, err)
				require.Equal(t, balance, foa.Balance())

				v := foa.Withdraw(balance)
				require.NoError(t, err)
				require.Equal(t, balance, v.Balance())

				require.NoError(t, err)
				require.Equal(t, zeroBalance, foa.Balance())

				events := backend.Events()
				require.Len(t, events, 4)

				// transaction event
				event := events[0]
				assert.Equal(t, event.Type, types.EventTypeTransactionExecuted)
				ret := types.TransactionExecutedPayload{}
				err = rlp.Decode(bytes.NewReader(event.Payload), &ret)
				require.NoError(t, err)
				// TODO: decode encoded tx and check for the amount and value
				// assert.Equal(t, foa.Address(), ret.Address)
				// assert.Equal(t, balance, ret.Amount)

				// block event
				event = events[1]
				assert.Equal(t, event.Type, types.EventTypeBlockExecuted)

				// transaction event
				event = events[2]
				assert.Equal(t, event.Type, types.EventTypeTransactionExecuted)
				ret = types.TransactionExecutedPayload{}
				err = rlp.Decode(bytes.NewReader(event.Payload), &ret)
				require.NoError(t, err)
				// TODO: decode encoded tx and check for the amount and value
				// assert.Equal(t, foa.Address(), ret.Address)
				// assert.Equal(t, balance, ret.Amount)

				// block event
				event = events[3]
				assert.Equal(t, event.Type, types.EventTypeBlockExecuted)

				// check gas usage
				computationUsed, err := backend.ComputationUsed()
				require.NoError(t, err)
				require.Equal(t, types.DefaultDirectCallBaseGasUsage*2, computationUsed)
			})
		})
	})

	t.Run("test withdraw (unhappy case)", func(t *testing.T) {
		t.Parallel()

		testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
			testutils.RunWithTestFlowEVMRootAddress(t, backend, func(rootAddr flow.Address) {
				testutils.RunWithEOATestAccount(t, backend, rootAddr, func(eoa *testutils.EOATestAccount) {
					bs, err := handler.NewBlockStore(backend, rootAddr)
					require.NoError(t, err)

					aa, err := handler.NewAddressAllocator(backend, rootAddr)
					require.NoError(t, err)

					// Withdraw calls are only possible within FOA accounts
					assertPanic(t, types.IsAUnAuthroizedMethodCallError, func() {
						em := &testutils.TestEmulator{}

						handler := handler.NewContractHandler(flowTokenAddress, bs, aa, backend, em)

						account := handler.AccountByAddress(testutils.RandomAddress(t), false)
						account.Withdraw(types.Balance(1))
					})

					// test insufficient total supply
					assertPanic(t, types.IsAInsufficientTotalSupplyError, func() {
						em := &testutils.TestEmulator{
							DirectCallFunc: func(call *types.DirectCall) (*types.Result, error) {
								return &types.Result{}, types.NewEVMExecutionError(fmt.Errorf("some sort of error"))
							},
						}

						handler := handler.NewContractHandler(flowTokenAddress, bs, aa, backend, em)
						account := handler.AccountByAddress(testutils.RandomAddress(t), true)

						account.Withdraw(types.Balance(1))
					})

					// test non fatal error of emulator
					assertPanic(t, types.IsEVMExecutionError, func() {
						em := &testutils.TestEmulator{
							DirectCallFunc: func(call *types.DirectCall) (*types.Result, error) {
								return &types.Result{}, types.NewEVMExecutionError(fmt.Errorf("some sort of error"))
							},
						}

						handler := handler.NewContractHandler(flowTokenAddress, bs, aa, backend, em)
						account := handler.AccountByAddress(testutils.RandomAddress(t), true)

						account.Withdraw(types.Balance(0))
					})

					// test fatal error of emulator
					assertPanic(t, types.IsAFatalError, func() {
						em := &testutils.TestEmulator{
							DirectCallFunc: func(call *types.DirectCall) (*types.Result, error) {
								return &types.Result{}, types.NewFatalError(fmt.Errorf("some sort of fatal error"))
							},
						}

						handler := handler.NewContractHandler(flowTokenAddress, bs, aa, backend, em)
						account := handler.AccountByAddress(testutils.RandomAddress(t), true)

						account.Withdraw(types.Balance(0))
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
					bs, err := handler.NewBlockStore(backend, rootAddr)
					require.NoError(t, err)

					aa, err := handler.NewAddressAllocator(backend, rootAddr)
					require.NoError(t, err)

					// test non fatal error of emulator
					assertPanic(t, types.IsEVMExecutionError, func() {
						em := &testutils.TestEmulator{
							DirectCallFunc: func(call *types.DirectCall) (*types.Result, error) {
								return &types.Result{}, types.NewEVMExecutionError(fmt.Errorf("some sort of error"))
							},
						}

						handler := handler.NewContractHandler(flowTokenAddress, bs, aa, backend, em)
						account := handler.AccountByAddress(testutils.RandomAddress(t), true)

						account.Deposit(types.NewFlowTokenVault(1))
					})

					// test fatal error of emulator
					assertPanic(t, types.IsAFatalError, func() {
						em := &testutils.TestEmulator{
							DirectCallFunc: func(call *types.DirectCall) (*types.Result, error) {
								return &types.Result{}, types.NewFatalError(fmt.Errorf("some sort of fatal error"))
							},
						}

						handler := handler.NewContractHandler(flowTokenAddress, bs, aa, backend, em)
						account := handler.AccountByAddress(testutils.RandomAddress(t), true)

						account.Deposit(types.NewFlowTokenVault(1))
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
				_, handler := SetupHandler(t, backend, rootAddr)

				foa := handler.AccountByAddress(handler.AllocateAddress(), true)
				require.NotNil(t, foa)

				// deposit 10000 flow
				orgBalance, err := types.NewBalanceFromAttoFlow(new(big.Int).Mul(big.NewInt(1e18), big.NewInt(10000)))
				require.NoError(t, err)
				vault := types.NewFlowTokenVault(orgBalance)
				foa.Deposit(vault)

				testContract := testutils.GetStorageTestContract(t)
				addr := foa.Deploy(testContract.ByteCode, math.MaxUint64, types.Balance(0))
				require.NotNil(t, addr)

				num := big.NewInt(22)

				_ = foa.Call(
					addr,
					testContract.MakeCallData(t, "store", num),
					math.MaxUint64,
					types.Balance(0))

				ret := foa.Call(
					addr,
					testContract.MakeCallData(t, "retrieve"),
					math.MaxUint64,
					types.Balance(0))

				require.Equal(t, num, new(big.Int).SetBytes(ret))
			})
		})
	})

	// TODO add test with test emulator for unhappy cases (emulator)
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

func SetupHandler(t testing.TB, backend types.Backend, rootAddr flow.Address) (*database.Database, *handler.ContractHandler) {
	bs, err := handler.NewBlockStore(backend, rootAddr)
	require.NoError(t, err)

	aa, err := handler.NewAddressAllocator(backend, rootAddr)
	require.NoError(t, err)

	db, err := database.NewDatabase(backend, rootAddr)
	require.NoError(t, err)

	emulator := emulator.NewEmulator(db)

	handler := handler.NewContractHandler(flowTokenAddress, bs, aa, backend, emulator)
	return db, handler
}
