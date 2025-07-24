package flow_test

import (
	"testing"

	"github.com/onflow/go-ethereum/rlp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/fingerprint"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestTransaction_SignatureOrdering(t *testing.T) {

	proposerAddress := unittest.RandomAddressFixture()
	proposerKeyIndex := uint32(1)
	proposerSequenceNumber := uint64(42)
	proposerSignature := []byte{1, 2, 3}

	authorizerAddress := unittest.RandomAddressFixture()
	authorizerKeyIndex := uint32(0)
	authorizerSignature := []byte{4, 5, 6}

	payerAddress := unittest.RandomAddressFixture()
	payerKeyIndex := uint32(0)
	payerSignature := []byte{7, 8, 9}

	tx, err := flow.NewTransactionBodyBuilder().
		SetScript([]byte(`transaction(){}`)).
		SetProposalKey(proposerAddress, proposerKeyIndex, proposerSequenceNumber).
		AddPayloadSignature(proposerAddress, proposerKeyIndex, proposerSignature).
		SetPayer(payerAddress).
		AddEnvelopeSignature(payerAddress, payerKeyIndex, payerSignature).
		AddAuthorizer(authorizerAddress).
		AddPayloadSignature(authorizerAddress, authorizerKeyIndex, authorizerSignature).
		Build()
	require.NoError(t, err)

	require.Len(t, tx.PayloadSignatures, 2)

	signatureA := tx.PayloadSignatures[0]
	signatureB := tx.PayloadSignatures[1]

	assert.Equal(t, proposerAddress, signatureA.Address)
	assert.Equal(t, authorizerAddress, signatureB.Address)
}

func TestTransaction_Status(t *testing.T) {
	statuses := map[flow.TransactionStatus]string{
		flow.TransactionStatusUnknown:   "UNKNOWN",
		flow.TransactionStatusPending:   "PENDING",
		flow.TransactionStatusFinalized: "FINALIZED",
		flow.TransactionStatusExecuted:  "EXECUTED",
		flow.TransactionStatusSealed:    "SEALED",
		flow.TransactionStatusExpired:   "EXPIRED",
	}

	for status, value := range statuses {
		assert.Equal(t, status.String(), value)
	}
}

// TestTransactionBodyID_Malleability provides basic validation that [flow.TransactionBody] is not malleable.
func TestTransactionBodyID_Malleability(t *testing.T) {
	txbody := unittest.TransactionBodyFixture()
	unittest.RequireEntityNonMalleable(t, &txbody, unittest.WithTypeGenerator[flow.TransactionSignature](func() flow.TransactionSignature {
		return unittest.TransactionSignatureFixture()
	}))
}

// TestTransactionBody_Fingerprint provides basic validation that the [TransactionBody] fingerprint
// is equivalent to its canonical RLP encoding.
func TestTransactionBody_Fingerprint(t *testing.T) {
	txbody := unittest.TransactionBodyFixture()
	fp1 := txbody.Fingerprint()
	fp2 := fingerprint.Fingerprint(txbody)
	fp3, err := rlp.EncodeToBytes(txbody)
	require.NoError(t, err)
	assert.Equal(t, fp1, fp2)
	assert.Equal(t, fp2, fp3)
}

// TestNewTransactionBody verifies that NewTransactionBody constructs a valid TransactionBody
// when given all required fields, and returns an error if any mandatory field is missing.
//
// Test Cases:
//
// 1. Valid input:
//   - Payer is non-empty and Script is non-empty.
//   - Ensures a TransactionBody is returned with all fields populated correctly.
//
// 2. Empty Payer:
//   - Payer is flow.EmptyAddress.
//   - Ensures an error is returned mentioning "Payer address must not be empty".
//
// 3. Empty Script:
//   - Script slice is empty.
//   - Ensures an error is returned mentioning "Script must not be empty".
func TestNewTransactionBody(t *testing.T) {
	t.Run("valid input", func(t *testing.T) {
		utb := UntrustedTransactionBodyFixture()

		tb, err := flow.NewTransactionBody(utb)
		assert.NoError(t, err)
		assert.NotNil(t, tb)

		assert.Equal(t, flow.TransactionBody(utb), *tb)
	})

	t.Run("empty Payer", func(t *testing.T) {
		utb := UntrustedTransactionBodyFixture(func(u *flow.UntrustedTransactionBody) {
			u.Payer = flow.EmptyAddress
		})

		tb, err := flow.NewTransactionBody(utb)
		assert.Error(t, err)
		assert.Nil(t, tb)
		assert.Contains(t, err.Error(), "Payer address must not be empty")
	})

	t.Run("empty Script", func(t *testing.T) {
		utb := UntrustedTransactionBodyFixture(func(u *flow.UntrustedTransactionBody) {
			u.Script = []byte{}
		})

		tb, err := flow.NewTransactionBody(utb)
		assert.Error(t, err)
		assert.Nil(t, tb)
		assert.Contains(t, err.Error(), "Script must not be empty")
	})
}

// TestNewSystemChunkTransactionBody verifies the behavior of the NewSystemChunkTransactionBody constructor.
// Test Cases:
//
// 1. Valid input:
//   - Script non-empty, GasLimit > 0, and Authorizers non-empty.
//   - Ensures a TransactionBody is returned with all fields populated correctly.
//
// 2. Empty Script:
//   - Script slice is empty.
//   - Ensures an error is returned mentioning "Script must not be empty in system chunk transaction".
//
// 3. Zero GasLimit:
//   - GasLimit == 0.
//   - Ensures an error is returned mentioning "Compute limit must not be empty in system chunk transaction".
//
// 4. Empty Authorizers:
//   - Authorizers slice is nil or empty.
//   - Ensures an error is returned mentioning "Authorizers must not be empty in system chunk transaction".
func TestNewSystemChunkTransactionBody(t *testing.T) {
	t.Run("valid input", func(t *testing.T) {
		utb := UntrustedTransactionBodyFixture()

		tb, err := flow.NewSystemChunkTransactionBody(utb)
		assert.NoError(t, err)
		assert.NotNil(t, tb)
		assert.Equal(t, flow.TransactionBody(utb), *tb)
	})

	t.Run("empty Script", func(t *testing.T) {
		utb := UntrustedTransactionBodyFixture(func(u *flow.UntrustedTransactionBody) {
			u.Script = []byte{}
		})

		tb, err := flow.NewSystemChunkTransactionBody(utb)
		assert.Error(t, err)
		assert.Nil(t, tb)
		assert.Contains(t, err.Error(), "Script must not be empty in system chunk transaction")
	})

	t.Run("zero GasLimit", func(t *testing.T) {
		utb := UntrustedTransactionBodyFixture(func(u *flow.UntrustedTransactionBody) {
			u.GasLimit = 0
		})

		tb, err := flow.NewSystemChunkTransactionBody(utb)
		assert.Error(t, err)
		assert.Nil(t, tb)
		assert.Contains(t, err.Error(), "Compute limit must not be empty in system chunk transaction")
	})

	t.Run("empty Authorizers", func(t *testing.T) {
		utb := UntrustedTransactionBodyFixture(func(u *flow.UntrustedTransactionBody) {
			u.Authorizers = []flow.Address{}
		})

		tb, err := flow.NewSystemChunkTransactionBody(utb)
		assert.Error(t, err)
		assert.Nil(t, tb)
		assert.Contains(t, err.Error(), "Authorizers must not be empty in system chunk transaction")
	})
}

// UntrustedTransactionBodyFixture returns an UntrustedTransactionBody
// pre‐populated with sane defaults. Any opts override those defaults.
func UntrustedTransactionBodyFixture(opts ...func(*flow.UntrustedTransactionBody)) flow.UntrustedTransactionBody {
	u := flow.UntrustedTransactionBody(unittest.TransactionBodyFixture())
	for _, opt := range opts {
		opt(&u)
	}
	return u
}
