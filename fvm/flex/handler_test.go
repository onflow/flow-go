package flex_test

import (
	"math"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"
	"github.com/onflow/flow-go/fvm/flex"
	"github.com/onflow/flow-go/fvm/flex/models"
	"github.com/onflow/flow-go/fvm/flex/testutils"
	"github.com/onflow/flow-go/model/flow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TODO add test for fatal errors

func TestFlexContractHandler(t *testing.T) {
	t.Parallel()
	t.Run("test last executed block call", func(t *testing.T) {
		testutils.RunWithTestBackend(t, func(backend models.Backend) {
			testutils.RunWithTestFlexRoot(t, backend, func(flexRoot flow.Address) {
				handler := flex.NewFlexContractHandler(backend, flexRoot)
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
				handler := flex.NewFlexContractHandler(backend, flexRoot)
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
				handler := flex.NewFlexContractHandler(backend, flexRoot)

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

					handler := flex.NewFlexContractHandler(backend, flexRoot)
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

					assert.PanicsWithError(t, models.ErrInsufficientComputation.Error(), func() {
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
				handler := flex.NewFlexContractHandler(backend, flexRoot)
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
				handler := flex.NewFlexContractHandler(backend, flexRoot)
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
