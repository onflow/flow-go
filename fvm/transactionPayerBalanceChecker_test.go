package fvm_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence"

	"github.com/onflow/flow-go/fvm"
	fvmmock "github.com/onflow/flow-go/fvm/environment/mock"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/storage/testutils"
	"github.com/onflow/flow-go/model/flow"
)

func TestTransactionPayerBalanceChecker(t *testing.T) {
	payer := flow.HexToAddress("1")
	t.Run("TransactionFeesEnabled == false disables the balance check", func(t *testing.T) {
		env := &fvmmock.Environment{}
		env.On("TransactionFeesEnabled").Return(false)

		proc := &fvm.TransactionProcedure{}
		proc.Transaction = &flow.TransactionBody{}
		proc.Transaction.Payer = payer

		txnState := testutils.NewSimpleTransaction(nil)

		d := fvm.TransactionPayerBalanceChecker{}
		maxFees, err := d.CheckPayerBalanceAndReturnMaxFees(proc, txnState, env)
		require.NoError(t, err)
		require.Equal(t, uint64(0), maxFees)
	})

	t.Run("errors during CheckPayerBalanceAndGetMaxTxFees invocation are wrapped and returned", func(t *testing.T) {
		someError := fmt.Errorf("some error")

		env := &fvmmock.Environment{}
		env.On("TransactionFeesEnabled").Return(true)
		env.On("CheckPayerBalanceAndGetMaxTxFees", mock.Anything, mock.Anything, mock.Anything).Return(
			nil,
			someError)

		proc := &fvm.TransactionProcedure{}
		proc.Transaction = &flow.TransactionBody{}
		proc.Transaction.Payer = payer

		txnState := testutils.NewSimpleTransaction(nil)

		d := fvm.TransactionPayerBalanceChecker{}
		maxFees, err := d.CheckPayerBalanceAndReturnMaxFees(proc, txnState, env)
		require.Error(t, err)
		require.True(t, errors.HasFailureCode(err, errors.FailureCodePayerBalanceCheckFailure))
		require.ErrorIs(t, err, someError)
		require.Equal(t, uint64(0), maxFees)
	})

	t.Run("unexpected result type from CheckPayerBalanceAndGetMaxTxFees causes error", func(t *testing.T) {
		env := &fvmmock.Environment{}
		env.On("TransactionFeesEnabled").Return(true)
		env.On("CheckPayerBalanceAndGetMaxTxFees", mock.Anything, mock.Anything, mock.Anything).Return(
			cadence.Struct{},
			nil)

		proc := &fvm.TransactionProcedure{}
		proc.Transaction = &flow.TransactionBody{}
		proc.Transaction.Payer = payer

		txnState := testutils.NewSimpleTransaction(nil)

		d := fvm.TransactionPayerBalanceChecker{}
		maxFees, err := d.CheckPayerBalanceAndReturnMaxFees(proc, txnState, env)
		require.Error(t, err)
		require.True(t, errors.HasFailureCode(err, errors.FailureCodePayerBalanceCheckFailure))
		require.Equal(t, uint64(0), maxFees)
	})

	t.Run("if payer can pay return max fees", func(t *testing.T) {
		env := &fvmmock.Environment{}
		env.On("TransactionFeesEnabled").Return(true)
		env.On("CheckPayerBalanceAndGetMaxTxFees", mock.Anything, mock.Anything, mock.Anything).Return(
			cadence.Struct{
				Fields: []cadence.Value{
					cadence.NewBool(true),
					cadence.UFix64(100),
					cadence.UFix64(100),
				},
			},
			nil)

		proc := &fvm.TransactionProcedure{}
		proc.Transaction = &flow.TransactionBody{}
		proc.Transaction.Payer = payer

		txnState := testutils.NewSimpleTransaction(nil)

		d := fvm.TransactionPayerBalanceChecker{}
		maxFees, err := d.CheckPayerBalanceAndReturnMaxFees(proc, txnState, env)
		require.NoError(t, err)
		require.Equal(t, uint64(100), maxFees)
	})

	t.Run("if payer cannot pay return insufficient balance error", func(t *testing.T) {
		env := &fvmmock.Environment{}
		env.On("TransactionFeesEnabled").Return(true)
		env.On("CheckPayerBalanceAndGetMaxTxFees", mock.Anything, mock.Anything, mock.Anything).Return(
			cadence.Struct{
				Fields: []cadence.Value{
					cadence.NewBool(false),
					cadence.UFix64(100),
					cadence.UFix64(101),
				},
			},
			nil)

		proc := &fvm.TransactionProcedure{}
		proc.Transaction = &flow.TransactionBody{}
		proc.Transaction.Payer = payer

		txnState := testutils.NewSimpleTransaction(nil)

		d := fvm.TransactionPayerBalanceChecker{}
		maxFees, err := d.CheckPayerBalanceAndReturnMaxFees(proc, txnState, env)
		require.Error(t, err)
		require.Contains(t, err.Error(), errors.NewInsufficientPayerBalanceError(payer, 100).Error())
		require.Equal(t, uint64(0), maxFees)
	})
}
