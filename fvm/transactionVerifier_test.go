package fvm_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/crypto"
	"github.com/onflow/flow-go/fvm/errors"
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

		txVerifier := fvm.NewTransactionSignatureVerifier(1000)
		err = txVerifier.Process(nil, &fvm.Context{}, proc, sth, programs.NewEmptyPrograms())
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

		txVerifier := fvm.NewTransactionSignatureVerifier(1000)
		err = txVerifier.Process(nil, &fvm.Context{}, proc, sth, programs.NewEmptyPrograms())
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
		txVerifier := fvm.NewTransactionSignatureVerifier(1000)
		err = txVerifier.Process(nil, &fvm.Context{}, proc, sth, programs.NewEmptyPrograms())
		require.Error(t, err)

		var envelopeError *errors.InvalidEnvelopeSignatureError
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
		txVerifier := fvm.NewTransactionSignatureVerifier(1000)
		err = txVerifier.Process(nil, &fvm.Context{}, proc, sth, programs.NewEmptyPrograms())
		require.Error(t, err)

		var payloadError *errors.InvalidPayloadSignatureError
		require.ErrorAs(t, err, &payloadError)
	})

	t.Run("invalid payload and envelope signatures", func(t *testing.T) {
		// TODO: this test is invalid and skipped.
		// The test will become valid once the FVM updates the order of validating signatures:
		// envelope needs to be checked first and payload later.
		t.Skip()
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
		txVerifier := fvm.NewTransactionSignatureVerifier(1000)
		err = txVerifier.Process(nil, &fvm.Context{}, proc, sth, programs.NewEmptyPrograms())
		require.Error(t, err)

		var payloadError *errors.InvalidEnvelopeSignatureError
		require.ErrorAs(t, err, &payloadError)
	})
}
