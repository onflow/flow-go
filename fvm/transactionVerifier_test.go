package fvm_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/crypto"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/storage"
	"github.com/onflow/flow-go/fvm/storage/testutils"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestTransactionVerification(t *testing.T) {
	t.Parallel()

	txnState := testutils.NewSimpleTransaction(nil)
	accounts := environment.NewAccounts(txnState)

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

	run := func(
		body *flow.TransactionBody,
		ctx fvm.Context,
		txn storage.TransactionPreparer,
	) error {
		executor := fvm.Transaction(body, 0).NewExecutor(ctx, txn)
		err := fvm.Run(executor)
		require.NoError(t, err)
		return executor.Output().Err
	}

	t.Run("duplicated authorization signatures", func(t *testing.T) {

		sig := flow.TransactionSignature{
			Address:     address1,
			SignerIndex: 0,
			KeyIndex:    0,
		}

		tx := &flow.TransactionBody{
			ProposalKey: flow.ProposalKey{
				Address:        address1,
				KeyIndex:       0,
				SequenceNumber: 0,
			},
			Payer:             address1,
			PayloadSignatures: []flow.TransactionSignature{sig, sig},
		}

		ctx := fvm.NewContext(
			fvm.WithAuthorizationChecksEnabled(true),
			fvm.WithAccountKeyWeightThreshold(1000),
			fvm.WithSequenceNumberCheckAndIncrementEnabled(false),
			fvm.WithTransactionBodyExecutionEnabled(false))
		err = run(tx, ctx, txnState)
		require.ErrorContains(
			t,
			err,
			"duplicate signatures are provided for the same key")
	})
	t.Run("duplicated authorization and envelope signatures", func(t *testing.T) {

		sig := flow.TransactionSignature{
			Address:     address1,
			SignerIndex: 0,
			KeyIndex:    0,
		}

		tx := &flow.TransactionBody{
			ProposalKey: flow.ProposalKey{
				Address:        address1,
				KeyIndex:       0,
				SequenceNumber: 0,
			},
			Payer:              address1,
			PayloadSignatures:  []flow.TransactionSignature{sig},
			EnvelopeSignatures: []flow.TransactionSignature{sig},
		}

		ctx := fvm.NewContext(
			fvm.WithAuthorizationChecksEnabled(true),
			fvm.WithAccountKeyWeightThreshold(1000),
			fvm.WithSequenceNumberCheckAndIncrementEnabled(false),
			fvm.WithTransactionBodyExecutionEnabled(false))
		err = run(tx, ctx, txnState)
		require.ErrorContains(
			t,
			err,
			"duplicate signatures are provided for the same key")
	})

	t.Run("invalid envelope signature", func(t *testing.T) {
		tx := &flow.TransactionBody{
			ProposalKey: flow.ProposalKey{
				Address:        address1,
				KeyIndex:       0,
				SequenceNumber: 0,
			},
			Payer: address2,
		}

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

		ctx := fvm.NewContext(
			fvm.WithAuthorizationChecksEnabled(true),
			fvm.WithAccountKeyWeightThreshold(1000),
			fvm.WithSequenceNumberCheckAndIncrementEnabled(false),
			fvm.WithTransactionBodyExecutionEnabled(false))
		err = run(tx, ctx, txnState)
		require.Error(t, err)
		require.True(t, errors.IsInvalidEnvelopeSignatureError(err))
	})

	t.Run("invalid payload signature", func(t *testing.T) {

		sig1 := flow.TransactionSignature{
			Address:     address1,
			SignerIndex: 0,
			KeyIndex:    0,
			// invalid signature
		}

		tx := &flow.TransactionBody{
			ProposalKey: flow.ProposalKey{
				Address:        address1,
				KeyIndex:       0,
				SequenceNumber: 0,
			},
			Payer: address2,
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

		ctx := fvm.NewContext(
			fvm.WithAuthorizationChecksEnabled(true),
			fvm.WithAccountKeyWeightThreshold(1000),
			fvm.WithSequenceNumberCheckAndIncrementEnabled(false),
			fvm.WithTransactionBodyExecutionEnabled(false))
		err = run(tx, ctx, txnState)
		require.Error(t, err)
		require.True(t, errors.IsInvalidPayloadSignatureError(err))
	})

	t.Run("invalid payload and envelope signatures", func(t *testing.T) {
		// TODO: this test expects a Payload error but should be updated to expect en Envelope error.
		// The test should be updated once the FVM updates the order of validating signatures:
		// envelope needs to be checked first and payload later.

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

		tx := &flow.TransactionBody{
			ProposalKey: flow.ProposalKey{
				Address:        address1,
				KeyIndex:       0,
				SequenceNumber: 0,
			},
			Payer:              address2,
			PayloadSignatures:  []flow.TransactionSignature{sig1},
			EnvelopeSignatures: []flow.TransactionSignature{sig2},
		}

		ctx := fvm.NewContext(
			fvm.WithAuthorizationChecksEnabled(true),
			fvm.WithAccountKeyWeightThreshold(1000),
			fvm.WithSequenceNumberCheckAndIncrementEnabled(false),
			fvm.WithTransactionBodyExecutionEnabled(false))
		err = run(tx, ctx, txnState)
		require.Error(t, err)

		// TODO: update to InvalidEnvelopeSignatureError once FVM verifier is updated.
		require.True(t, errors.IsInvalidPayloadSignatureError(err))
	})

	// test that Transaction Signature verification uses the correct domain tag for verification
	// i.e the message verification reconstruction logic uses the right tag (check signatureContinuation.verify() )
	t.Run("tag combinations", func(t *testing.T) {
		cases := []struct {
			signTag  string
			validity bool
		}{
			{
				signTag:  string(flow.TransactionDomainTag[:]), // only valid tag
				validity: true,
			},
			{
				signTag:  "", // invalid tag
				validity: false,
			}, {
				signTag:  "random_tag", // invalid tag
				validity: false,
			},
		}

		sig := flow.TransactionSignature{
			Address:     address1,
			SignerIndex: 0,
			KeyIndex:    0,
		}

		tx := &flow.TransactionBody{
			ProposalKey: flow.ProposalKey{
				Address:        address1,
				KeyIndex:       0,
				SequenceNumber: 0,
			},
			Payer:              address1,
			EnvelopeSignatures: []flow.TransactionSignature{sig},
		}

		for _, c := range cases {
			t.Run(fmt.Sprintf("sign tag: %v", c.signTag), func(t *testing.T) {

				// generate an envelope signature using the test tag
				hasher, err := crypto.NewPrefixedHashing(privKey1.HashAlgo, c.signTag)
				require.NoError(t, err)
				sig, err := privKey1.PrivateKey.Sign(tx.EnvelopeMessage(), hasher)
				require.NoError(t, err)

				// set the signature into the transaction
				tx.EnvelopeSignatures[0].Signature = sig

				ctx := fvm.NewContext(
					fvm.WithAuthorizationChecksEnabled(true),
					fvm.WithAccountKeyWeightThreshold(1000),
					fvm.WithSequenceNumberCheckAndIncrementEnabled(false),
					fvm.WithTransactionBodyExecutionEnabled(false))
				err = run(tx, ctx, txnState)
				if c.validity {
					require.NoError(t, err)
				} else {
					require.Error(t, err)
					require.True(t, errors.IsInvalidEnvelopeSignatureError(err))
				}
			})
		}
	})
}
