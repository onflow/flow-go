package fvm_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/storage/testutils"
	"github.com/onflow/flow-go/fvm/tracing"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestTransactionSequenceNumProcess(t *testing.T) {
	t.Run("sequence number update (happy path)", func(t *testing.T) {
		txnState := testutils.NewSimpleTransaction(nil)
		accounts := environment.NewAccounts(txnState)

		// create an account
		address := flow.HexToAddress("1234")
		privKey, err := unittest.AccountKeyDefaultFixture()
		require.NoError(t, err)
		err = accounts.Create([]flow.AccountPublicKey{privKey.PublicKey(1000)}, address)
		require.NoError(t, err)

		tx := flow.TransactionBody{
			ProposalKey: flow.ProposalKey{
				Address:        address,
				KeyIndex:       0,
				SequenceNumber: 0,
			},
		}
		proc := fvm.Transaction(&tx, 0)

		seqChecker := fvm.TransactionSequenceNumberChecker{}
		err = seqChecker.CheckAndIncrementSequenceNumber(
			tracing.NewTracerSpan(),
			proc,
			txnState)
		require.NoError(t, err)

		// get fetch the sequence number and it should be updated
		seqNumber, err := accounts.GetAccountPublicKeySequenceNumber(address, 0)
		require.NoError(t, err)
		require.Equal(t, seqNumber, uint64(1))
	})
	t.Run("invalid sequence number", func(t *testing.T) {
		txnState := testutils.NewSimpleTransaction(nil)
		accounts := environment.NewAccounts(txnState)

		// create an account
		address := flow.HexToAddress("1234")
		privKey, err := unittest.AccountKeyDefaultFixture()
		require.NoError(t, err)
		err = accounts.Create([]flow.AccountPublicKey{privKey.PublicKey(1000)}, address)
		require.NoError(t, err)

		tx := flow.TransactionBody{
			// invalid sequence number is 2
			ProposalKey: flow.ProposalKey{
				Address:        address,
				KeyIndex:       0,
				SequenceNumber: 2,
			},
		}
		proc := fvm.Transaction(&tx, 0)

		seqChecker := fvm.TransactionSequenceNumberChecker{}
		err = seqChecker.CheckAndIncrementSequenceNumber(
			tracing.NewTracerSpan(),
			proc,
			txnState)
		require.Error(t, err)
		require.True(t, errors.HasErrorCode(err, errors.ErrCodeInvalidProposalSeqNumberError))

		// get fetch the sequence number and check it to be  unchanged
		seqNumber, err := accounts.GetAccountPublicKeySequenceNumber(address, 0)
		require.NoError(t, err)
		require.Equal(t, seqNumber, uint64(0))
	})
	t.Run("invalid address", func(t *testing.T) {
		txnState := testutils.NewSimpleTransaction(nil)
		accounts := environment.NewAccounts(txnState)

		// create an account
		address := flow.HexToAddress("1234")
		privKey, err := unittest.AccountKeyDefaultFixture()
		require.NoError(t, err)
		err = accounts.Create([]flow.AccountPublicKey{privKey.PublicKey(1000)}, address)
		require.NoError(t, err)

		// wrong address
		tx := flow.TransactionBody{
			ProposalKey: flow.ProposalKey{
				Address:        flow.HexToAddress("2222"),
				KeyIndex:       0,
				SequenceNumber: 0,
			},
		}
		proc := fvm.Transaction(&tx, 0)

		seqChecker := &fvm.TransactionSequenceNumberChecker{}
		err = seqChecker.CheckAndIncrementSequenceNumber(
			tracing.NewTracerSpan(),
			proc,
			txnState)
		require.Error(t, err)

		// get fetch the sequence number and check it to be unchanged
		seqNumber, err := accounts.GetAccountPublicKeySequenceNumber(address, 0)
		require.NoError(t, err)
		require.Equal(t, seqNumber, uint64(0))
	})
}
