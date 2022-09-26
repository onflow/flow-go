package fvm_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/crypto"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/fvm/utils"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestTransactionVerification(t *testing.T) {
	ledger := utils.NewSimpleView()
	stTxn := state.NewStateTransaction(ledger, state.DefaultParameters())
	accounts := environment.NewAccounts(stTxn)

	// create 2 accounts
	address1 := flow.HexToAddress("1234")
	privKey1, err := unittest.AccountKeyDefaultFixture()
	require.NoError(t, err)

	err = accounts.Create([]flow.AccountPublicKey{privKey1.PublicKey(1000)}, address1)
	require.NoError(t, err)

	address2 := flow.HexToAddress("1235")
	privKey2, err := unittest.AccountKeyDefaultFixture()
	require.NoError(t, err)

	err = accounts.Create([]flow.AccountPublicKey{privKey2.PublicKey(1000)}, address2)
	require.NoError(t, err)

	tx := flow.TransactionBody{}

	t.Run("duplicated authorization signatures", func(t *testing.T) {

		sig := flow.TransactionSignature{
			Address:     address1,
			SignerIndex: 0,
			KeyIndex:    0,
		}

		tx.SetProposalKey(address1, 0, 0)
		tx.SetPayer(address1)

		tx.PayloadSignatures = []flow.TransactionSignature{sig, sig}
		proc := fvm.Transaction(&tx, 0)

		txVerifier := fvm.NewTransactionVerifier(1000)
		err = txVerifier.Process(fvm.Context{}, proc, stTxn, nil)
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), "duplicate signatures are provided for the same key"))
	})
	t.Run("duplicated authorization and envelope signatures", func(t *testing.T) {

		sig := flow.TransactionSignature{
			Address:     address1,
			SignerIndex: 0,
			KeyIndex:    0,
		}

		tx.SetProposalKey(address1, 0, 0)
		tx.SetPayer(address1)

		tx.PayloadSignatures = []flow.TransactionSignature{sig}
		tx.EnvelopeSignatures = []flow.TransactionSignature{sig}
		proc := fvm.Transaction(&tx, 0)

		txVerifier := fvm.NewTransactionVerifier(1000)
		err = txVerifier.Process(fvm.Context{}, proc, stTxn, nil)
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), "duplicate signatures are provided for the same key"))
	})

	t.Run("invalid envelope signature", func(t *testing.T) {
		tx.SetProposalKey(address1, 0, 0)
		tx.SetPayer(address2)

		// assign a valid payload signature
		hasher1, err := crypto.NewPrefixedHashing(privKey1.HashAlgo, flow.TransactionTagString)
		require.NoError(t, err)
		validSig, err := privKey1.PrivateKey.Sign(tx.PayloadMessage(), hasher1) // valid signature
		require.NoError(t, err)

		sig1 := flow.TransactionSignature{
			Address:     address1,
			SignerIndex: 0,
			KeyIndex:    0,
			Signature:   validSig,
		}

		sig2 := flow.TransactionSignature{
			Address:     address2,
			SignerIndex: 0,
			KeyIndex:    0,
			// invalid signature
		}

		tx.PayloadSignatures = []flow.TransactionSignature{sig1}
		tx.EnvelopeSignatures = []flow.TransactionSignature{sig2}

		proc := fvm.Transaction(&tx, 0)
		txVerifier := fvm.NewTransactionVerifier(1000)
		err = txVerifier.Process(fvm.Context{}, proc, stTxn, nil)
		require.Error(t, err)

		var envelopeError errors.InvalidEnvelopeSignatureError
		require.ErrorAs(t, err, &envelopeError)
	})

	t.Run("invalid payload signature", func(t *testing.T) {
		tx.SetProposalKey(address1, 0, 0)
		tx.SetPayer(address2)

		sig1 := flow.TransactionSignature{
			Address:     address1,
			SignerIndex: 0,
			KeyIndex:    0,
			// invalid signature
		}

		// assign a valid envelope signature
		hasher2, err := crypto.NewPrefixedHashing(privKey2.HashAlgo, flow.TransactionTagString)
		require.NoError(t, err)
		validSig, err := privKey2.PrivateKey.Sign(tx.EnvelopeMessage(), hasher2) // valid signature
		require.NoError(t, err)

		sig2 := flow.TransactionSignature{
			Address:     address2,
			SignerIndex: 0,
			KeyIndex:    0,
			Signature:   validSig,
		}

		tx.PayloadSignatures = []flow.TransactionSignature{sig1}
		tx.EnvelopeSignatures = []flow.TransactionSignature{sig2}

		proc := fvm.Transaction(&tx, 0)
		txVerifier := fvm.NewTransactionVerifier(1000)
		err = txVerifier.Process(fvm.Context{}, proc, stTxn, nil)
		require.Error(t, err)

		var payloadError errors.InvalidPayloadSignatureError
		require.ErrorAs(t, err, &payloadError)
	})

	t.Run("invalid payload and envelope signatures", func(t *testing.T) {
		// TODO: this test expects a Payload error but should be updated to expect en Envelope error.
		// The test should be updated once the FVM updates the order of validating signatures:
		// envelope needs to be checked first and payload later.
		tx.SetProposalKey(address1, 0, 0)
		tx.SetPayer(address2)

		sig1 := flow.TransactionSignature{
			Address:     address1,
			SignerIndex: 0,
			KeyIndex:    0,
			// invalid signature
		}

		sig2 := flow.TransactionSignature{
			Address:     address2,
			SignerIndex: 0,
			KeyIndex:    0,
			// invalid signature
		}

		tx.PayloadSignatures = []flow.TransactionSignature{sig1}
		tx.EnvelopeSignatures = []flow.TransactionSignature{sig2}

		proc := fvm.Transaction(&tx, 0)
		txVerifier := fvm.NewTransactionVerifier(1000)
		err = txVerifier.Process(fvm.Context{}, proc, stTxn, nil)
		require.Error(t, err)

		// TODO: update to InvalidEnvelopeSignatureError once FVM verifier is updated.
		var payloadError errors.InvalidPayloadSignatureError
		require.ErrorAs(t, err, &payloadError)
	})

	t.Run("frozen account is rejected", func(t *testing.T) {

		txChecker := fvm.NewTransactionVerifier(-1)

		frozenAddress, notFrozenAddress, st := makeTwoAccounts(t, nil, nil)
		accounts := environment.NewAccounts(st)

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
		err = txChecker.Process(fvm.Context{}, tx, st, nil)
		require.NoError(t, err)

		tx = fvm.Transaction(&flow.TransactionBody{
			Payer:       notFrozenAddress,
			ProposalKey: flow.ProposalKey{Address: notFrozenAddress},
			Authorizers: []flow.Address{notFrozenAddress},
		}, 0)
		err = txChecker.Process(fvm.Context{}, tx, st, nil)
		require.NoError(t, err)

		tx = fvm.Transaction(&flow.TransactionBody{
			Payer:       notFrozenAddress,
			ProposalKey: flow.ProposalKey{Address: notFrozenAddress},
			Authorizers: []flow.Address{frozenAddress},
		}, 0)
		err = txChecker.Process(fvm.Context{}, tx, st, nil)
		require.Error(t, err)

		// all addresses must not be frozen
		tx = fvm.Transaction(&flow.TransactionBody{
			Payer:       notFrozenAddress,
			ProposalKey: flow.ProposalKey{Address: notFrozenAddress},
			Authorizers: []flow.Address{frozenAddress, notFrozenAddress},
		}, 0)
		err = txChecker.Process(fvm.Context{}, tx, st, nil)
		require.Error(t, err)

		// Payer should be part of authorizers account, but lets check it separately for completeness

		tx = fvm.Transaction(&flow.TransactionBody{
			Payer:       notFrozenAddress,
			ProposalKey: flow.ProposalKey{Address: notFrozenAddress},
		}, 0)
		err = txChecker.Process(fvm.Context{}, tx, st, nil)
		require.NoError(t, err)

		tx = fvm.Transaction(&flow.TransactionBody{
			Payer:       frozenAddress,
			ProposalKey: flow.ProposalKey{Address: notFrozenAddress},
		}, 0)
		err = txChecker.Process(fvm.Context{}, tx, st, nil)
		require.Error(t, err)

		// Proposal account

		tx = fvm.Transaction(&flow.TransactionBody{
			Payer:       notFrozenAddress,
			ProposalKey: flow.ProposalKey{Address: frozenAddress},
		}, 0)
		err = txChecker.Process(fvm.Context{}, tx, st, nil)
		require.Error(t, err)

		tx = fvm.Transaction(&flow.TransactionBody{
			Payer:       notFrozenAddress,
			ProposalKey: flow.ProposalKey{Address: notFrozenAddress},
		}, 0)
		err = txChecker.Process(fvm.Context{}, tx, st, nil)
		require.NoError(t, err)
	})
}
