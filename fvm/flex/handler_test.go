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
// TODO update test to just use a test emulator

func TestFlexContractHandler(t *testing.T) {
	t.Parallel()
	t.Run("test last executed block call", func(t *testing.T) {
		testutils.RunWithTestBackend(t, func(backend models.Backend) {
			testutils.RunWithTestFlexRoot(t, backend, func(flexRoot flow.Address) {

				db, err := storage.NewDatabase(backend, flexRoot)
				require.NoError(t, err)

				emulator := evm.NewEmulator(db)

				handler := flex.NewFlexContractHandler(db, backend, emulator)
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

	t.Run("test foa creation", func(t *testing.T) {
		testutils.RunWithTestBackend(t, func(backend models.Backend) {
			testutils.RunWithTestFlexRoot(t, backend, func(flexRoot flow.Address) {
				db, err := storage.NewDatabase(backend, flexRoot)
				require.NoError(t, err)

				emulator := evm.NewEmulator(db)

				handler := flex.NewFlexContractHandler(db, backend, emulator)
				foa := handler.AllocateAddress()
				require.NotNil(t, foa)

				expectedAddress := models.NewFlexAddress(common.HexToAddress("0x00000000000000000001"))
				require.Equal(t, expectedAddress, foa)
			})
		})
	})

	t.Run("test running transaction", func(t *testing.T) {
		testutils.RunWithTestBackend(t, func(backend models.Backend) {
			testutils.RunWithTestFlexRoot(t, backend, func(flexRoot flow.Address) {
				db, err := storage.NewDatabase(backend, flexRoot)
				require.NoError(t, err)

				emulator := evm.NewEmulator(db)

				handler := flex.NewFlexContractHandler(db, backend, emulator)

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

	t.Run("test gas compliance", func(t *testing.T) {
		testutils.RunWithTestBackend(t, func(backend models.Backend) {
			testutils.RunWithTestFlexRoot(t, backend, func(flexRoot flow.Address) {
				testutils.RunWithEOATestAccount(t, backend, flexRoot, func(eoa *testutils.EOATestAccount) {

					db, err := storage.NewDatabase(backend, flexRoot)
					require.NoError(t, err)

					emulator := evm.NewEmulator(db)

					handler := flex.NewFlexContractHandler(db, backend, emulator)
					// set tx limit above the tx limit

					gasLimit := uint64(testutils.TestComputationLimit + 1)
					tx := eoa.PrepareSignAndEncodeTx(
						t,
						common.Address{},
						nil,
						nil,
						gasLimit,
						big.NewInt(1e8), // high gas fee to test coinbase collection,
					)

					assertPanic(t, false, func() {
						handler.Run(tx, eoa.FlexAddress())
					})
				})
			})
		})
	})
}

func TestFOA(t *testing.T) {
	t.Run("test deposit/withdraw", func(t *testing.T) {
		testutils.RunWithTestBackend(t, func(backend models.Backend) {
			testutils.RunWithTestFlexRoot(t, backend, func(flexRoot flow.Address) {
				db, err := storage.NewDatabase(backend, flexRoot)
				require.NoError(t, err)

				emulator := evm.NewEmulator(db)

				handler := flex.NewFlexContractHandler(db, backend, emulator)
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
			})
		})
	})

	t.Run("test deploy/call", func(t *testing.T) {
		testutils.RunWithTestBackend(t, func(backend models.Backend) {
			testutils.RunWithTestFlexRoot(t, backend, func(flexRoot flow.Address) {
				db, err := storage.NewDatabase(backend, flexRoot)
				require.NoError(t, err)

				emulator := evm.NewEmulator(db)

				handler := flex.NewFlexContractHandler(db, backend, emulator)
				foa := handler.AccountByAddress(handler.AllocateAddress(), true)
				require.NotNil(t, foa)

				// deposit 100 flow
				orgBalance, err := models.NewBalanceFromAttoFlow(new(big.Int).Mul(big.NewInt(1e18), big.NewInt(100)))
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
}

func TestHandler_TransactionRun(t *testing.T) {

	t.Run("test - transaction run (happy case)", func(t *testing.T) {
		testutils.RunWithTestBackend(t, func(backend models.Backend) {
			testutils.RunWithTestFlexRoot(t, backend, func(flexRoot flow.Address) {
				testutils.RunWithEOATestAccount(t, backend, flexRoot, func(eoa *testutils.EOATestAccount) {

					db, err := storage.NewDatabase(backend, flexRoot)
					require.NoError(t, err)

					result := &models.Result{
						StateRootHash:           testutils.RandomCommonHash(),
						LogsRootHash:            testutils.RandomCommonHash(),
						DeployedContractAddress: models.FlexAddress(testutils.RandomAddress()),
						ReturnedValue:           testutils.RandomData(),
						GasConsumed:             testutils.RandomGas(1000),
						Logs: []*types.Log{
							testutils.GetRandomLogFixture(),
							testutils.GetRandomLogFixture(),
						},
					}

					em := &testutils.TestEmulator{
						RunTransactionFunc: func(tx *types.Transaction, coinbase models.FlexAddress) (*models.Result, error) {
							return result, nil
						},
					}
					handler := flex.NewFlexContractHandler(db, backend, em)

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
					require.Len(t, events, len(result.Logs)+1)
					for i, l := range result.Logs {
						assert.Equal(t, events[i].Type, models.EventTypeFlexEVMLog)
						retLog := types.Log{}
						err := rlp.Decode(bytes.NewReader(events[i].Payload), &retLog)
						require.NoError(t, err)
						assert.Equal(t, *l, retLog)
					}

					// check block event
					lastEvent := events[len(events)-1]
					assert.Equal(t, lastEvent.Type, models.EventTypeFlexBlockExecuted)
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

					db, err := storage.NewDatabase(backend, flexRoot)
					require.NoError(t, err)

					em := &testutils.TestEmulator{
						RunTransactionFunc: func(tx *types.Transaction, coinbase models.FlexAddress) (*models.Result, error) {
							return nil, models.NewEVMExecutionError(fmt.Errorf("some sort of error"))
						},
					}
					handler := flex.NewFlexContractHandler(db, backend, em)

					coinbase := models.NewFlexAddress(common.Address{})

					// test RLP decoding (non fatal)
					assertPanic(t, false, func() {
						// invalid RLP encoding
						invalidTx := "badencoding"
						handler.Run([]byte(invalidTx), coinbase)
					})

					// test gas limit (non fatal)
					assertPanic(t, false, func() {
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
}

func assertPanic(t *testing.T, isFatal bool, f func()) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("The code did not panic")
		}
		err, ok := r.(error)
		if !ok {
			t.Fatal("panic is not with an error type")
		}
		require.False(t, errors.IsFailure(err))
	}()
	f()
}
