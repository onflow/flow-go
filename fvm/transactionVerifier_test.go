package fvm_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/fvm/utils"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestTransactionVerification(t *testing.T) {
	ledger := utils.NewSimpleView()
	sth := state.NewStateHolder(state.NewState(ledger))
	accounts := state.NewAccounts(sth)

	// create an account
	address := flow.HexToAddress("1234")
	privKey, err := unittest.AccountKeyDefaultFixture()
	require.NoError(t, err)
	err = accounts.Create([]flow.AccountPublicKey{privKey.PublicKey(1000)}, address)
	require.NoError(t, err)

	sig1 := flow.TransactionSignature{
		Address:     address,
		SignerIndex: 0,
		KeyIndex:    0,
		Signature:   []byte("SIGNATURE"),
	}

	sig2 := flow.TransactionSignature{
		Address:     address,
		SignerIndex: 1,
		KeyIndex:    0,
		Signature:   []byte("SIGNATURE"),
	}

	t.Run("duplicated authorization signatures", func(t *testing.T) {
		tx := flow.TransactionBody{}
		tx.SetProposalKey(address, 0, 0)
		tx.SetPayer(address)
		tx.PayloadSignatures = []flow.TransactionSignature{sig1, sig2}
		proc := fvm.Transaction(&tx, 0)

		txVerifier := fvm.NewTransactionSignatureVerifier(1000)
		err = txVerifier.Process(nil, &fvm.Context{}, proc, sth, programs.NewEmptyPrograms())
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), "duplicate signatures are provided for the same key"))
	})
	t.Run("duplicated authorization and envelope signatures", func(t *testing.T) {
		tx := flow.TransactionBody{}
		tx.SetProposalKey(address, 0, 0)
		tx.SetPayer(address)

		tx.PayloadSignatures = []flow.TransactionSignature{sig1}
		tx.EnvelopeSignatures = []flow.TransactionSignature{sig2}
		proc := fvm.Transaction(&tx, 0)

		txVerifier := fvm.NewTransactionSignatureVerifier(1000)
		err = txVerifier.Process(nil, &fvm.Context{}, proc, sth, programs.NewEmptyPrograms())
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), "duplicate signatures are provided for the same key"))
	})
}
