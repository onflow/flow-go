package fvm_test

import (
	"fmt"
	"testing"

	"github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/fvm/utils"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
)

func TestTransactionSequenceNumProcess(t *testing.T) {
	t.Run("sequence number update (happy path)", func(t *testing.T) {
		ledger := utils.NewSimpleView()
		sth := state.NewStateHolder(state.NewState(ledger))
		accounts := state.NewAccounts(sth)

		// create an account
		address := flow.HexToAddress("1234")
		privKey, err := unittest.AccountKeyDefaultFixture()
		require.NoError(t, err)
		err = accounts.Create([]flow.AccountPublicKey{privKey.PublicKey(1000)}, address)

		tx := flow.TransactionBody{}
		tx.SetProposalKey(address, 0, 0)
		err = signEnvelope(&tx, address, *privKey)
		require.NoError(t, err)
		proc := fvm.Transaction(&tx, 0)

		seqChecker := &fvm.TransactionSequenceNumberChecker{}
		err = seqChecker.Process(nil, &fvm.Context{}, proc, sth, programs.NewEmptyPrograms())
		require.NoError(t, err)

		// get fetch the sequence number and it should be updated
		key, err := accounts.GetPublicKey(address, 0)
		require.NoError(t, err)
		require.Equal(t, key.SeqNumber, uint64(1))
	})
	t.Run("invalid sequence number", func(t *testing.T) {
		ledger := utils.NewSimpleView()
		sth := state.NewStateHolder(state.NewState(ledger))
		accounts := state.NewAccounts(sth)

		// create an account
		address := flow.HexToAddress("1234")
		privKey, err := unittest.AccountKeyDefaultFixture()
		require.NoError(t, err)
		err = accounts.Create([]flow.AccountPublicKey{privKey.PublicKey(1000)}, address)

		tx := flow.TransactionBody{}
		// invalid sequence number is 2
		tx.SetProposalKey(address, 0, 2)
		err = signEnvelope(&tx, address, *privKey)
		require.NoError(t, err)
		proc := fvm.Transaction(&tx, 0)

		seqChecker := &fvm.TransactionSequenceNumberChecker{}
		err = seqChecker.Process(nil, &fvm.Context{}, proc, sth, programs.NewEmptyPrograms())
		require.Error(t, err)
		require.Equal(t, err.(errors.Error).Code(), errors.ErrCodeInvalidProposalSeqNumberError)

		// get fetch the sequence number and check it to be  unchanged
		key, err := accounts.GetPublicKey(address, 0)
		require.NoError(t, err)
		require.Equal(t, key.SeqNumber, uint64(0))
	})
	t.Run("invalid address", func(t *testing.T) {
		ledger := utils.NewSimpleView()
		sth := state.NewStateHolder(state.NewState(ledger))
		accounts := state.NewAccounts(sth)

		// create an account
		address := flow.HexToAddress("1234")
		privKey, err := unittest.AccountKeyDefaultFixture()
		require.NoError(t, err)
		err = accounts.Create([]flow.AccountPublicKey{privKey.PublicKey(1000)}, address)

		tx := flow.TransactionBody{}
		// wrong address
		tx.SetProposalKey(flow.HexToAddress("2222"), 0, 0)
		proc := fvm.Transaction(&tx, 0)

		seqChecker := &fvm.TransactionSequenceNumberChecker{}
		err = seqChecker.Process(nil, &fvm.Context{}, proc, sth, programs.NewEmptyPrograms())
		require.Error(t, err)

		// get fetch the sequence number and check it to be unchanged
		key, err := accounts.GetPublicKey(address, 0)
		require.NoError(t, err)
		require.Equal(t, key.SeqNumber, uint64(0))
	})

}

func signEnvelope(tx *flow.TransactionBody, account flow.Address, privateKey flow.AccountPrivateKey) error {
	hasher, err := crypto.NewHasher(privateKey.HashAlgo)
	if err != nil {
		return fmt.Errorf("failed to create hasher: %w", err)
	}

	err = tx.SignEnvelope(account, 0, privateKey.PrivateKey, hasher)

	if err != nil {
		return fmt.Errorf("failed to sign transaction: %w", err)
	}

	return nil
}
