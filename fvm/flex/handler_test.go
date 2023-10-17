package flex_test

import (
	"bytes"
	"fmt"
	"math"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/flex"
	"github.com/onflow/flow-go/fvm/flex/evm"
	"github.com/onflow/flow-go/fvm/flex/models"
	"github.com/onflow/flow-go/fvm/flex/storage"
	"github.com/onflow/flow-go/fvm/flex/testutils"
	"github.com/onflow/flow-go/model/flow"
)

// TODO add test for fatal errors

func TestHandler_TransactionRun(t *testing.T) {
	t.Parallel()

	t.Run("test - transaction run (happy case)", func(t *testing.T) {
		testutils.RunWithTestBackend(t, func(backend models.Backend) {
			testutils.RunWithTestFlexRoot(t, backend, func(flexRoot flow.Address) {
				testutils.RunWithEOATestAccount(t, backend, flexRoot, func(eoa *testutils.EOATestAccount) {

					bs, err := flex.NewBlockStore(backend, flexRoot)
					require.NoError(t, err)

					result := &models.Result{
						StateRootHash:           testutils.RandomCommonHash(),
						DeployedContractAddress: models.FlexAddress(testutils.RandomAddress()),
						ReturnedValue:           testutils.RandomData(),
						GasConsumed:             testutils.RandomGas(1000),
						Logs: []*types.Log{
							testutils.GetRandomLogFixture(),
							testutils.GetRandomLogFixture(),
						},
					}

					em := &testutils.TestEmulator{
						RunTransactionFunc: func(tx *types.Transaction) (*models.Result, error) {
							return result, nil
						},
					}
					handler := flex.NewFlexContractHandler(bs, backend, em)

					coinbase := models.NewFlexAddress(common.Address{})

					tx := eoa.PrepareSignAndEncodeTx(
						t,
						common.Address{},
						nil,
						nil,
						100_000,
						big.NewInt(1),
					)
					success := handler.Run(tx, coinbase)
					require.True(t, success)

					// check gas usage
					computationUsed, err := backend.ComputationUsed()
					require.NoError(t, err)
					require.Equal(t, result.GasConsumed, computationUsed)

					// check events (1 extra for block event)
					events := backend.Events()
					// require.Len(t, events, len(result.Logs)+1)
					// for i, l := range result.Logs {
					// 	assert.Equal(t, events[i].Type, models.EventTypeFlexEVMLog)
					// 	retLog := types.Log{}
					// 	err := rlp.Decode(bytes.NewReader(events[i].Payload), &retLog)
					// 	require.NoError(t, err)
					// 	assert.Equal(t, *l, retLog)
					// }

					// check block event
					lastEvent := events[len(events)-1]
					assert.Equal(t, lastEvent.Type, models.EventTypeBlockExecuted)
					payload := models.BlockExecutedEventPayload{}
					err = rlp.Decode(bytes.NewReader(lastEvent.Payload), &payload)
					require.NoError(t, err)
				})
			})
		})
	})

	t.Run("test - transaction run (unhappy cases)", func(t *testing.T) {
		testutils.RunWithTestBackend(t, func(backend models.Backend) {
			testutils.RunWithTestFlexRoot(t, backend, func(flexRoot flow.Address) {
				testutils.RunWithEOATestAccount(t, backend, flexRoot, func(eoa *testutils.EOATestAccount) {

					bs, err := flex.NewBlockStore(backend, flexRoot)
					require.NoError(t, err)

					em := &testutils.TestEmulator{
						RunTransactionFunc: func(tx *types.Transaction) (*models.Result, error) {
							return &models.Result{}, models.NewEVMExecutionError(fmt.Errorf("some sort of error"))
						},
					}
					handler := flex.NewFlexContractHandler(bs, backend, em)

					coinbase := models.NewFlexAddress(common.Address{})

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
							common.Address{},
							nil,
							nil,
							gasLimit,
							big.NewInt(1),
						)

						handler.Run([]byte(tx), coinbase)
					})

					// tx execution failure
					// TODO: if not using bool, we should expect panic
					tx := eoa.PrepareSignAndEncodeTx(
						t,
						common.Address{},
						nil,
						nil,
						100_000,
						big.NewInt(1),
					)

					success := handler.Run([]byte(tx), coinbase)
					require.False(t, success)
				})
			})
		})
	})

	t.Run("test running transaction (with integrated emulator)", func(t *testing.T) {
		testutils.RunWithTestBackend(t, func(backend models.Backend) {
			testutils.RunWithTestFlexRoot(t, backend, func(flexRoot flow.Address) {

				bs, err := flex.NewBlockStore(backend, flexRoot)
				require.NoError(t, err)

				db, err := storage.NewDatabase(backend, flexRoot)
				require.NoError(t, err)

				emulator := evm.NewEmulator(db)

				handler := flex.NewFlexContractHandler(bs, backend, emulator)

				eoa := testutils.GetTestEOAAccount(t, testutils.EOATestAccount1KeyHex)

				// deposit 1 Flow to the foa account
				addr := handler.AllocateAddress()
				orgBalance, err := models.NewBalanceFromAttoFlow(big.NewInt(1e18))
				require.NoError(t, err)
				vault := models.NewFlowTokenVault(orgBalance)
				foa := handler.AccountByAddress(addr, true)
				foa.Deposit(vault)

				// transfer 0.1 flow to the non-foa address
				deduction, err := models.NewBalanceFromAttoFlow(big.NewInt(1e17))
				require.NoError(t, err)
				foa.Call(eoa.FlexAddress(), nil, 400000, deduction)
				require.Equal(t, orgBalance.Sub(deduction), foa.Balance())

				// transfer 0.01 flow back to the foa through
				addition, err := models.NewBalanceFromAttoFlow(big.NewInt(1e16))
				require.NoError(t, err)

				tx := eoa.PrepareSignAndEncodeTx(
					t,
					foa.Address().ToCommon(),
					nil,
					addition.ToAttoFlow(),
					params.TxGas*10,
					big.NewInt(1e8), // high gas fee to test coinbase collection,
				)

				// setup coinbase
				foa2 := handler.AllocateAddress()
				account2 := handler.AccountByAddress(foa2, true)
				require.Equal(t, models.Balance(0), account2.Balance())

				success := handler.Run(tx, account2.Address())
				require.True(t, success)
				require.Equal(t, orgBalance.Sub(deduction).Add(addition), foa.Balance())

				// fees has been collected to the coinbase
				require.NotEqual(t, models.Balance(0), account2.Balance())

			})
		})
	})
}

func TestHandler_OpsWithoutEmulator(t *testing.T) {
	t.Parallel()

	t.Run("test last executed block call", func(t *testing.T) {
		testutils.RunWithTestBackend(t, func(backend models.Backend) {
			testutils.RunWithTestFlexRoot(t, backend, func(flexRoot flow.Address) {
				bs, err := flex.NewBlockStore(backend, flexRoot)
				require.NoError(t, err)

				handler := flex.NewFlexContractHandler(bs, backend, nil)
				// test call last executed block without initialization
				b := handler.LastExecutedBlock()
				require.Equal(t, models.GenesisFlexBlock, b)

				// do some changes
				addr := handler.AllocateAddress()
				require.NotNil(t, addr)

				// check if uuid index and block height has been incremented
				b = handler.LastExecutedBlock()
				require.Equal(t, uint64(1), b.Height)
				require.Equal(t, uint64(2), b.UUIDIndex)
			})
		})
	})

	t.Run("test address allocation", func(t *testing.T) {
		testutils.RunWithTestBackend(t, func(backend models.Backend) {
			testutils.RunWithTestFlexRoot(t, backend, func(flexRoot flow.Address) {
				blockchain, err := flex.NewBlockStore(backend, flexRoot)
				require.NoError(t, err)

				handler := flex.NewFlexContractHandler(blockchain, backend, nil)
				foa := handler.AllocateAddress()
				require.NotNil(t, foa)

				expectedAddress := models.NewFlexAddress(common.HexToAddress("0x00000000000000000001"))
				require.Equal(t, expectedAddress, foa)
			})
		})
	})
}

func TestHandler_FOA(t *testing.T) {

	t.Run("test deposit/withdraw (with integrated emulator)", func(t *testing.T) {
		testutils.RunWithTestBackend(t, func(backend models.Backend) {
			testutils.RunWithTestFlexRoot(t, backend, func(flexRoot flow.Address) {
				bs, err := flex.NewBlockStore(backend, flexRoot)
				require.NoError(t, err)

				db, err := storage.NewDatabase(backend, flexRoot)
				require.NoError(t, err)

				emulator := evm.NewEmulator(db)

				handler := flex.NewFlexContractHandler(bs, backend, emulator)
				foa := handler.AccountByAddress(handler.AllocateAddress(), true)
				require.NotNil(t, foa)

				zeroBalance, err := models.NewBalanceFromAttoFlow(big.NewInt(0))
				require.NoError(t, err)
				require.Equal(t, zeroBalance, foa.Balance())

				balance, err := models.NewBalanceFromAttoFlow(big.NewInt(100))
				require.NoError(t, err)
				vault := models.NewFlowTokenVault(balance)

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

				// deposit event
				event := events[0]
				assert.Equal(t, event.Type, models.EventTypeFlowTokenDeposit)
				ret := models.FlowTokenEventPayload{}
				err = rlp.Decode(bytes.NewReader(event.Payload), &ret)
				require.NoError(t, err)
				assert.Equal(t, foa.Address(), ret.Address)
				assert.Equal(t, balance, ret.Amount)

				// block event
				event = events[1]
				assert.Equal(t, event.Type, models.EventTypeBlockExecuted)

				// withdraw event
				event = events[2]
				assert.Equal(t, event.Type, models.EventTypeFlowTokenWithdrawal)
				ret = models.FlowTokenEventPayload{}
				err = rlp.Decode(bytes.NewReader(event.Payload), &ret)
				require.NoError(t, err)
				assert.Equal(t, foa.Address(), ret.Address)
				assert.Equal(t, balance, ret.Amount)

				// block event
				event = events[3]
				assert.Equal(t, event.Type, models.EventTypeBlockExecuted)

				// check gas usage
				computationUsed, err := backend.ComputationUsed()
				require.NoError(t, err)
				require.Equal(t, models.DefaultDirectCallBaseGasUsage*2, computationUsed)
			})
		})
	})

	t.Run("test withdraw (unhappy case)", func(t *testing.T) {
		testutils.RunWithTestBackend(t, func(backend models.Backend) {
			testutils.RunWithTestFlexRoot(t, backend, func(flexRoot flow.Address) {
				testutils.RunWithEOATestAccount(t, backend, flexRoot, func(eoa *testutils.EOATestAccount) {
					bs, err := flex.NewBlockStore(backend, flexRoot)
					require.NoError(t, err)

					// Withdraw calls are only possible within FOA accounts
					assertPanic(t, models.IsAUnAuthroizedMethodCallError, func() {
						em := &testutils.TestEmulator{}
						handler := flex.NewFlexContractHandler(bs, backend, em)

						account := handler.AccountByAddress(testutils.RandomFlexAddress(), false)
						account.Withdraw(models.Balance(1))
					})

					// test insufficient total supply
					assertPanic(t, models.IsAInsufficientTotalSupplyError, func() {
						em := &testutils.TestEmulator{
							WithdrawFromFunc: func(address models.FlexAddress, amount *big.Int) (*models.Result, error) {
								return &models.Result{}, models.NewEVMExecutionError(fmt.Errorf("some sort of error"))
							},
						}
						handler := flex.NewFlexContractHandler(bs, backend, em)
						account := handler.AccountByAddress(testutils.RandomFlexAddress(), true)
						account.Withdraw(models.Balance(1))
					})

					// test non fatal error of emulator
					assertPanic(t, models.IsEVMExecutionError, func() {
						em := &testutils.TestEmulator{
							WithdrawFromFunc: func(address models.FlexAddress, amount *big.Int) (*models.Result, error) {
								return &models.Result{}, models.NewEVMExecutionError(fmt.Errorf("some sort of error"))
							},
						}
						handler := flex.NewFlexContractHandler(bs, backend, em)
						account := handler.AccountByAddress(testutils.RandomFlexAddress(), true)
						account.Withdraw(models.Balance(0))
					})

					// test fatal error of emulator
					assertPanic(t, models.IsAFatalError, func() {
						em := &testutils.TestEmulator{
							WithdrawFromFunc: func(address models.FlexAddress, amount *big.Int) (*models.Result, error) {
								return &models.Result{}, models.NewFatalError(fmt.Errorf("some sort of fatal error"))
							},
						}
						handler := flex.NewFlexContractHandler(bs, backend, em)
						account := handler.AccountByAddress(testutils.RandomFlexAddress(), true)
						account.Withdraw(models.Balance(0))
					})
				})
			})
		})
	})

	t.Run("test deposit (unhappy case)", func(t *testing.T) {
		testutils.RunWithTestBackend(t, func(backend models.Backend) {
			testutils.RunWithTestFlexRoot(t, backend, func(flexRoot flow.Address) {
				testutils.RunWithEOATestAccount(t, backend, flexRoot, func(eoa *testutils.EOATestAccount) {
					bs, err := flex.NewBlockStore(backend, flexRoot)
					require.NoError(t, err)

					// test non fatal error of emulator
					assertPanic(t, models.IsEVMExecutionError, func() {
						em := &testutils.TestEmulator{
							MintToFunc: func(address models.FlexAddress, amount *big.Int) (*models.Result, error) {
								return nil, models.NewEVMExecutionError(fmt.Errorf("some sort of error"))
							},
						}
						handler := flex.NewFlexContractHandler(bs, backend, em)
						account := handler.AccountByAddress(testutils.RandomFlexAddress(), true)
						account.Deposit(models.NewFlowTokenVault(1))
					})

					// test fatal error of emulator
					assertPanic(t, models.IsAFatalError, func() {
						em := &testutils.TestEmulator{
							MintToFunc: func(address models.FlexAddress, amount *big.Int) (*models.Result, error) {
								return nil, models.NewFatalError(fmt.Errorf("some sort of fatal error"))
							},
						}
						handler := flex.NewFlexContractHandler(bs, backend, em)
						account := handler.AccountByAddress(testutils.RandomFlexAddress(), true)
						account.Deposit(models.NewFlowTokenVault(1))
					})
				})
			})
		})
	})

	t.Run("test deploy/call (with integrated emulator)", func(t *testing.T) {
		// TODO update this test with events, gas metering, etc
		testutils.RunWithTestBackend(t, func(backend models.Backend) {
			testutils.RunWithTestFlexRoot(t, backend, func(flexRoot flow.Address) {
				bs, err := flex.NewBlockStore(backend, flexRoot)
				require.NoError(t, err)

				db, err := storage.NewDatabase(backend, flexRoot)
				require.NoError(t, err)

				emulator := evm.NewEmulator(db)

				handler := flex.NewFlexContractHandler(bs, backend, emulator)
				foa := handler.AccountByAddress(handler.AllocateAddress(), true)
				require.NotNil(t, foa)

				// deposit 10000 flow
				orgBalance, err := models.NewBalanceFromAttoFlow(new(big.Int).Mul(big.NewInt(1e18), big.NewInt(10000)))
				require.NoError(t, err)
				vault := models.NewFlowTokenVault(orgBalance)
				foa.Deposit(vault)

				testContract := testutils.GetTestContract(t)
				addr := foa.Deploy(testContract.ByteCode, math.MaxUint64, models.Balance(0))
				require.NotNil(t, addr)

				num := big.NewInt(22)

				_ = foa.Call(
					addr,
					testContract.MakeStoreCallData(t, num),
					math.MaxUint64,
					models.Balance(0))

				ret := foa.Call(
					addr,
					testContract.MakeRetrieveCallData(t),
					math.MaxUint64,
					models.Balance(0))

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
