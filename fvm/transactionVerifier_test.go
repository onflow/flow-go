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

		txVerifier := fvm.NewTransactionVerifier(1000)
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

		txVerifier := fvm.NewTransactionVerifier(1000)
		err = txVerifier.Process(nil, &fvm.Context{}, proc, sth, programs.NewEmptyPrograms())
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), "duplicate signatures are provided for the same key"))
	})

	t.Run("frozen account is rejected", func(t *testing.T) {

		txChecker := fvm.NewTransactionVerifier(-1)

		frozenAddress, notFrozenAddress, st := makeTwoAccounts(t, nil, nil)
		accounts := state.NewAccounts(st)
		programsStorage := programs.NewEmptyPrograms()

		// freeze account
		err := accounts.SetAccountFrozen(frozenAddress, true)
		require.NoError(t, err)

		// make sure freeze status is correct
		frozen, err := accounts.GetAccountFrozen(frozenAddress)
		require.NoError(t, err)
		require.True(t, frozen)

		frozen, err = accounts.GetAccountFrozen(notFrozenAddress)
		require.NoError(t, err)
		require.False(t, frozen)

		// Authorizers
		tx := fvm.Transaction(&flow.TransactionBody{
			Payer:       notFrozenAddress,
			ProposalKey: flow.ProposalKey{Address: notFrozenAddress},
		}, 0)
		err = txChecker.Process(nil, &fvm.Context{}, tx, st, programsStorage)
		require.NoError(t, err)

		tx = fvm.Transaction(&flow.TransactionBody{
			Payer:       notFrozenAddress,
			ProposalKey: flow.ProposalKey{Address: notFrozenAddress},
			Authorizers: []flow.Address{notFrozenAddress},
		}, 0)
		err = txChecker.Process(nil, &fvm.Context{}, tx, st, programsStorage)
		require.NoError(t, err)

		tx = fvm.Transaction(&flow.TransactionBody{
			Payer:       notFrozenAddress,
			ProposalKey: flow.ProposalKey{Address: notFrozenAddress},
			Authorizers: []flow.Address{frozenAddress},
		}, 0)
		err = txChecker.Process(nil, &fvm.Context{}, tx, st, programsStorage)
		require.Error(t, err)

		// all addresses must not be frozen
		tx = fvm.Transaction(&flow.TransactionBody{
			Payer:       notFrozenAddress,
			ProposalKey: flow.ProposalKey{Address: notFrozenAddress},
			Authorizers: []flow.Address{frozenAddress, notFrozenAddress},
		}, 0)
		err = txChecker.Process(nil, &fvm.Context{}, tx, st, programsStorage)
		require.Error(t, err)

		// Payer should be part of authorizers account, but lets check it separately for completeness

		tx = fvm.Transaction(&flow.TransactionBody{
			Payer:       notFrozenAddress,
			ProposalKey: flow.ProposalKey{Address: notFrozenAddress},
		}, 0)
		err = txChecker.Process(nil, &fvm.Context{}, tx, st, programsStorage)
		require.NoError(t, err)

		tx = fvm.Transaction(&flow.TransactionBody{
			Payer:       frozenAddress,
			ProposalKey: flow.ProposalKey{Address: notFrozenAddress},
		}, 0)
		err = txChecker.Process(nil, &fvm.Context{}, tx, st, programsStorage)
		require.Error(t, err)

		// Proposal account

		tx = fvm.Transaction(&flow.TransactionBody{
			Payer:       notFrozenAddress,
			ProposalKey: flow.ProposalKey{Address: frozenAddress},
		}, 0)
		err = txChecker.Process(nil, &fvm.Context{}, tx, st, programsStorage)
		require.Error(t, err)

		tx = fvm.Transaction(&flow.TransactionBody{
			Payer:       notFrozenAddress,
			ProposalKey: flow.ProposalKey{Address: notFrozenAddress},
		}, 0)
		err = txChecker.Process(nil, &fvm.Context{}, tx, st, programsStorage)
		require.NoError(t, err)
	})
}
