package fvm_test

import (
	"fmt"
	"testing"

	"github.com/onflow/crypto/hash"
	"github.com/stretchr/testify/assert"
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

const fullWeight = 1000

func newContext() fvm.Context {
	return fvm.NewContext(
		flow.Mainnet.Chain(),
		fvm.WithAuthorizationChecksEnabled(true),
		fvm.WithAccountKeyWeightThreshold(fullWeight),
		fvm.WithSequenceNumberCheckAndIncrementEnabled(false),
		fvm.WithTransactionBodyExecutionEnabled(false))
}

func newAccount(t *testing.T, accounts *environment.StatefulAccounts) (flow.Address, *flow.AccountPrivateKey) {
	address := unittest.RandomAddressFixture()
	privKey, err := unittest.AccountKeyDefaultFixture()
	require.NoError(t, err)
	err = accounts.Create([]flow.AccountPublicKey{privKey.PublicKey(fullWeight)}, address)
	require.NoError(t, err)
	return address, privKey
}

func TestTransactionVerification(t *testing.T) {
	t.Parallel()

	txnState := testutils.NewSimpleTransaction(nil)
	accounts := environment.NewAccounts(txnState)

	// create 4 accounts
	address1, privKey1 := newAccount(t, accounts)
	address2, privKey2 := newAccount(t, accounts)
	address3, privKey3 := newAccount(t, accounts)

	// add a partial weight key for address1 for later tests
	err := accounts.AppendAccountPublicKey(address1, privKey1.PublicKey(fullWeight/2))
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

		ctx := newContext()
		err := run(tx, ctx, txnState)
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

		ctx := newContext()
		err := run(tx, ctx, txnState)
		require.ErrorContains(
			t,
			err,
			"duplicate signatures are provided for the same key")
	})

	t.Run("invalid envelope signature", func(t *testing.T) {
		proposer := address1
		payer := address2

		tx := &flow.TransactionBody{
			ProposalKey: flow.ProposalKey{
				Address:        proposer,
				KeyIndex:       0,
				SequenceNumber: 0,
			},
			Payer: payer,
		}

		// assign a valid payload signature
		hasher1, err := crypto.NewPrefixedHashing(privKey1.HashAlgo, flow.TransactionTagString)
		require.NoError(t, err)
		validSig, err := privKey1.PrivateKey.Sign(tx.PayloadMessage(), hasher1) // valid signature
		require.NoError(t, err)

		sig1 := flow.TransactionSignature{
			Address:     proposer,
			SignerIndex: 0,
			KeyIndex:    0,
			Signature:   validSig,
		}

		sig2 := flow.TransactionSignature{
			Address:     payer,
			SignerIndex: 0,
			KeyIndex:    0,
			// invalid signature
		}

		tx.PayloadSignatures = []flow.TransactionSignature{sig1}
		tx.EnvelopeSignatures = []flow.TransactionSignature{sig2}

		ctx := newContext()
		err = run(tx, ctx, txnState)

		require.True(t, errors.IsInvalidEnvelopeSignatureError(err))
		assert.ErrorContainsf(t, err, fmt.Sprintf("on account %s", payer), "should mention the proposer address %s", payer)
	})

	t.Run("invalid payload signature", func(t *testing.T) {

		proposer := address1
		payer := address2

		sig1 := flow.TransactionSignature{
			Address:     proposer,
			SignerIndex: 0,
			KeyIndex:    0,
			// invalid signature
		}

		tx := &flow.TransactionBody{
			ProposalKey: flow.ProposalKey{
				Address:        proposer,
				KeyIndex:       0,
				SequenceNumber: 0,
			},
			Payer: payer,
		}

		// assign a valid envelope signature
		hasher2, err := crypto.NewPrefixedHashing(privKey2.HashAlgo, flow.TransactionTagString)
		require.NoError(t, err)
		validSig, err := privKey2.PrivateKey.Sign(tx.EnvelopeMessage(), hasher2) // valid signature
		require.NoError(t, err)

		sig2 := flow.TransactionSignature{
			Address:     payer,
			SignerIndex: 0,
			KeyIndex:    0,
			Signature:   validSig,
		}

		tx.PayloadSignatures = []flow.TransactionSignature{sig1}
		tx.EnvelopeSignatures = []flow.TransactionSignature{sig2}

		ctx := newContext()
		err = run(tx, ctx, txnState)
		require.True(t, errors.IsInvalidPayloadSignatureError(err))
		assert.ErrorContainsf(t, err, fmt.Sprintf("on account %s", proposer), "should mention the proposer address %s", proposer)
	})

	t.Run("invalid payload and envelope signatures", func(t *testing.T) {
		// TODO: this test expects a Payload error but should be updated to expect en Envelope error.
		// The test should be updated once the FVM updates the order of validating signatures:
		// envelope needs to be checked first and payload later.

		proposer := address1
		payer := address2

		sig1 := flow.TransactionSignature{
			Address:     proposer,
			SignerIndex: 0,
			KeyIndex:    0,
			// invalid signature
		}

		sig2 := flow.TransactionSignature{
			Address:     payer,
			SignerIndex: 0,
			KeyIndex:    0,
			// invalid signature
		}

		tx := &flow.TransactionBody{
			ProposalKey: flow.ProposalKey{
				Address:        proposer,
				KeyIndex:       0,
				SequenceNumber: 0,
			},
			Payer:              address2,
			PayloadSignatures:  []flow.TransactionSignature{sig1},
			EnvelopeSignatures: []flow.TransactionSignature{sig2},
		}

		ctx := newContext()
		err := run(tx, ctx, txnState)

		// TODO: update to InvalidEnvelopeSignatureError once FVM verifier is updated.
		require.True(t, errors.IsInvalidPayloadSignatureError(err))
		assert.ErrorContainsf(t, err, fmt.Sprintf("on account %s", proposer), "should mention the proposer address %s", proposer)
	})

	t.Run("missing authorizer signatures", func(t *testing.T) {
		payer := address1
		address4 := unittest.RandomAddressFixture()
		authorizers := []flow.Address{address2, address3, address4}

		tx := &flow.TransactionBody{
			ProposalKey: flow.ProposalKey{
				Address:        payer,
				KeyIndex:       0,
				SequenceNumber: 0,
			},
			Payer:       payer,
			Authorizers: authorizers,
		}

		// assign a valid payload signature
		hasher, err := crypto.NewPrefixedHashing(hash.SHA3_256, flow.TransactionTagString)
		require.NoError(t, err)

		validSig, err := privKey2.PrivateKey.Sign(tx.PayloadMessage(), hasher) // valid signature
		require.NoError(t, err)
		sig2 := flow.TransactionSignature{
			Address:     address2,
			SignerIndex: 0,
			KeyIndex:    0,
			Signature:   validSig,
		}

		validSig, err = privKey3.PrivateKey.Sign(tx.PayloadMessage(), hasher) // valid signature
		require.NoError(t, err)
		sig3 := flow.TransactionSignature{
			Address:     address3,
			SignerIndex: 0,
			KeyIndex:    0,
			Signature:   validSig,
		}

		tx.PayloadSignatures = []flow.TransactionSignature{sig2, sig3} // address from address4 is missing

		validSig, err = privKey1.PrivateKey.Sign(tx.EnvelopeMessage(), hasher) // valid signature
		require.NoError(t, err)

		sig1 := flow.TransactionSignature{
			Address:     payer,
			SignerIndex: 0,
			KeyIndex:    0,
			Signature:   validSig,
		}

		tx.EnvelopeSignatures = []flow.TransactionSignature{sig1}

		ctx := newContext()
		err = run(tx, ctx, txnState)
		assert.ErrorContainsf(t, err, fmt.Sprintf("authorization failed for account %s", address4), "should mention an authorizer error")
	})

	t.Run("one authorizer with not enough weights", func(t *testing.T) {
		payer := address1
		authorizers := []flow.Address{address2, address3}

		// use partial weight key for address3
		err := accounts.AppendAccountPublicKey(address3, privKey3.PublicKey(fullWeight/2))
		require.NoError(t, err)

		tx := &flow.TransactionBody{
			ProposalKey: flow.ProposalKey{
				Address:        payer,
				KeyIndex:       0,
				SequenceNumber: 0,
			},
			Payer:       payer,
			Authorizers: authorizers,
		}

		// assign a valid payload signature
		hasher, err := crypto.NewPrefixedHashing(hash.SHA3_256, flow.TransactionTagString)
		require.NoError(t, err)

		validSig, err := privKey2.PrivateKey.Sign(tx.PayloadMessage(), hasher) // valid signature
		require.NoError(t, err)
		sig2 := flow.TransactionSignature{
			Address:     address2,
			SignerIndex: 0,
			KeyIndex:    0,
			Signature:   validSig,
		}

		validSig, err = privKey3.PrivateKey.Sign(tx.PayloadMessage(), hasher) // valid signature
		require.NoError(t, err)
		sig3 := flow.TransactionSignature{
			Address:     address3,
			SignerIndex: 0,
			KeyIndex:    1, // partial weight key
			Signature:   validSig,
		}

		tx.PayloadSignatures = []flow.TransactionSignature{sig2, sig3}
		validSig, err = privKey1.PrivateKey.Sign(tx.EnvelopeMessage(), hasher) // valid signature
		require.NoError(t, err)

		sig1 := flow.TransactionSignature{
			Address:     payer,
			SignerIndex: 0,
			KeyIndex:    0,
			Signature:   validSig,
		}

		tx.EnvelopeSignatures = []flow.TransactionSignature{sig1}

		ctx := newContext()
		err = run(tx, ctx, txnState)
		assert.ErrorContains(t, err, "authorizer account does not have sufficient signatures", "error should be about insufficient authorizer weights")
	})

	t.Run("payer with not enough weights", func(t *testing.T) {
		payer := address1

		tx := &flow.TransactionBody{
			ProposalKey: flow.ProposalKey{
				Address:        payer,
				KeyIndex:       1, // partial weight key
				SequenceNumber: 0,
			},
			Payer: payer,
		}

		// assign a valid payload signature
		hasher, err := crypto.NewPrefixedHashing(hash.SHA3_256, flow.TransactionTagString)
		require.NoError(t, err)

		validSig, err := privKey1.PrivateKey.Sign(tx.EnvelopeMessage(), hasher) // valid signature
		require.NoError(t, err)

		sig1 := flow.TransactionSignature{
			Address:     payer,
			SignerIndex: 0,
			KeyIndex:    1, // partial weight key
			Signature:   validSig,
		}

		tx.EnvelopeSignatures = []flow.TransactionSignature{sig1}

		ctx := newContext()
		err = run(tx, ctx, txnState)
		assert.ErrorContains(t, err, "payer account does not have sufficient signatures", "error should be about insufficient payer weights")
	})

	t.Run("payer signing the payload only", func(t *testing.T) {
		payer := address1

		tx := &flow.TransactionBody{
			ProposalKey: flow.ProposalKey{
				Address:        payer,
				KeyIndex:       0,
				SequenceNumber: 0,
			},
			Payer: payer,
		}

		// assign a valid payload signature
		hasher, err := crypto.NewPrefixedHashing(hash.SHA3_256, flow.TransactionTagString)
		require.NoError(t, err)

		validSig, err := privKey1.PrivateKey.Sign(tx.PayloadMessage(), hasher) // valid signature
		require.NoError(t, err)

		sig := flow.TransactionSignature{
			Address:     payer,
			SignerIndex: 0,
			KeyIndex:    0,
			Signature:   validSig,
		}

		tx.PayloadSignatures = []flow.TransactionSignature{sig}

		ctx := newContext()
		err = run(tx, ctx, txnState)
		assert.ErrorContains(t, err, "payer account does not have sufficient signatures", "error should be about insufficient payer weights")
	})

	t.Run("weights are checked before signatures", func(t *testing.T) {
		// use a key with partial weight and an invalid signature and make sure
		// the weight error is returned before the signature error
		payer := address1

		tx := &flow.TransactionBody{
			ProposalKey: flow.ProposalKey{
				Address:        payer,
				KeyIndex:       1, // partial weight key
				SequenceNumber: 0,
			},
			Payer: payer,
		}

		sig1 := flow.TransactionSignature{
			Address:     payer,
			SignerIndex: 0,
			KeyIndex:    1, // partial weight key
			// empty signature is invalid signature
		}

		tx.EnvelopeSignatures = []flow.TransactionSignature{sig1}

		ctx := newContext()
		err = run(tx, ctx, txnState)
		assert.ErrorContains(t, err, "payer account does not have sufficient signatures", "error should be about insufficient payer weights not invalid signature")
	})

	t.Run("signature from unrelated address", func(t *testing.T) {
		payer := address1
		proposer := address2
		authorizers := []flow.Address{address3}
		unrelated := unittest.RandomAddressFixture()

		tx := &flow.TransactionBody{
			ProposalKey: flow.ProposalKey{
				Address:        proposer,
				KeyIndex:       0,
				SequenceNumber: 0,
			},
			Payer:       payer,
			Authorizers: authorizers,
		}

		// proposer signature
		sig2 := flow.TransactionSignature{
			Address:     proposer,
			SignerIndex: 0,
			KeyIndex:    0,
		}

		// authorizer signature
		sig3 := flow.TransactionSignature{
			Address:     authorizers[0],
			SignerIndex: 0,
			KeyIndex:    0,
		}

		// unrelated account signature
		sig4 := flow.TransactionSignature{
			Address:     unrelated,
			SignerIndex: 0,
			KeyIndex:    0,
		}

		sig1 := flow.TransactionSignature{
			Address:     payer,
			SignerIndex: 0,
			KeyIndex:    0,
		}

		// unrelated account signature is included as a payload signature
		tx.PayloadSignatures = []flow.TransactionSignature{sig2, sig3, sig4}
		tx.EnvelopeSignatures = []flow.TransactionSignature{sig1}

		ctx := newContext()
		err = run(tx, ctx, txnState)
		assert.ErrorContains(t, err, "that is neither payer nor authorizer nor proposer", "error should be about unrelated account signature")

		// unrelated account signature is included as an envelope signature
		tx.PayloadSignatures = []flow.TransactionSignature{sig2, sig3}
		tx.EnvelopeSignatures = []flow.TransactionSignature{sig1, sig4}

		err = run(tx, ctx, txnState)
		assert.ErrorContains(t, err, "that is neither payer nor authorizer nor proposer", "error should be about unrelated account signature")
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

				ctx := newContext()
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
