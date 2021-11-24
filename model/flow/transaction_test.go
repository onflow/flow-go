package flow_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestTransaction_SignatureOrdering(t *testing.T) {
	tx := flow.NewTransactionBody()

	proposerAddress := unittest.RandomAddressFixture()
	proposerKeyIndex := uint64(1)
	proposerSequenceNumber := uint64(42)
	proposerSignature := []byte{1, 2, 3}

	authorizerAddress := unittest.RandomAddressFixture()
	authorizerKeyIndex := uint64(0)
	authorizerSignature := []byte{4, 5, 6}

	payerAddress := unittest.RandomAddressFixture()
	payerKeyIndex := uint64(0)
	payerSignature := []byte{7, 8, 9}

	tx.SetProposalKey(proposerAddress, proposerKeyIndex, proposerSequenceNumber)
	tx.AddPayloadSignature(proposerAddress, proposerKeyIndex, proposerSignature)

	tx.SetPayer(payerAddress)
	tx.AddEnvelopeSignature(payerAddress, payerKeyIndex, payerSignature)

	tx.AddAuthorizer(authorizerAddress)
	tx.AddPayloadSignature(authorizerAddress, authorizerKeyIndex, authorizerSignature)

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
