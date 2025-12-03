package convert_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestConvertIdentifier tests converting a flow.Identifier to and from a protobuf message.
func TestConvertIdentifier(t *testing.T) {
	t.Parallel()

	id := unittest.IdentifierFixture()

	msg := convert.IdentifierToMessage(id)
	converted := convert.MessageToIdentifier(msg)

	assert.Equal(t, id, converted)
}

// TestConvertIdentifiers tests converting a slice of flow.Identifiers to and from protobuf messages.
func TestConvertIdentifiers(t *testing.T) {
	t.Parallel()

	ids := unittest.IdentifierListFixture(10)

	msgs := convert.IdentifiersToMessages(ids)
	converted := convert.MessagesToIdentifiers(msgs)

	assert.Equal(t, ids, flow.IdentifierList(converted))
}

// TestConvertSignature tests converting a crypto.Signature to and from a protobuf message.
func TestConvertSignature(t *testing.T) {
	t.Parallel()

	sig := unittest.SignatureFixture()

	msg := convert.SignatureToMessage(sig)
	converted := convert.MessageToSignature(msg)

	assert.Equal(t, sig, converted)
}

// TestConvertSignatures tests converting a slice of crypto.Signatures to and from protobuf messages.
func TestConvertSignatures(t *testing.T) {
	t.Parallel()

	sigs := unittest.SignaturesFixture(5)

	msgs := convert.SignaturesToMessages(sigs)
	converted := convert.MessagesToSignatures(msgs)

	assert.Equal(t, sigs, converted)
}

// TestConvertStateCommitment tests converting a flow.StateCommitment to and from a protobuf message.
func TestConvertStateCommitment(t *testing.T) {
	t.Parallel()

	sc := unittest.StateCommitmentFixture()

	msg := convert.StateCommitmentToMessage(sc)
	converted, err := convert.MessageToStateCommitment(msg)
	require.NoError(t, err)

	assert.Equal(t, sc, converted)
}

// TestConvertStateCommitmentInvalidLength tests that MessageToStateCommitment returns an error
// for invalid length byte slices.
func TestConvertStateCommitmentInvalidLength(t *testing.T) {
	t.Parallel()

	invalidMsg := []byte{0x01, 0x02, 0x03} // Too short

	_, err := convert.MessageToStateCommitment(invalidMsg)
	assert.Error(t, err)
}

// TestConvertAggregatedSignatures tests converting a slice of flow.AggregatedSignatures to and from
// protobuf messages.
func TestConvertAggregatedSignatures(t *testing.T) {
	t.Parallel()

	aggSigs := []flow.AggregatedSignature{
		{
			SignerIDs:          unittest.IdentifierListFixture(3),
			VerifierSignatures: unittest.SignaturesFixture(3),
		},
		{
			SignerIDs:          unittest.IdentifierListFixture(5),
			VerifierSignatures: unittest.SignaturesFixture(5),
		},
	}

	msgs := convert.AggregatedSignaturesToMessages(aggSigs)
	converted := convert.MessagesToAggregatedSignatures(msgs)

	assert.Equal(t, aggSigs, converted)
}

// TestConvertAggregatedSignaturesEmpty tests converting an empty slice of flow.AggregatedSignatures.
func TestConvertAggregatedSignaturesEmpty(t *testing.T) {
	t.Parallel()

	aggSigs := []flow.AggregatedSignature{}

	msgs := convert.AggregatedSignaturesToMessages(aggSigs)
	converted := convert.MessagesToAggregatedSignatures(msgs)

	assert.Empty(t, converted)
}

// TestConvertChainId tests converting a valid chainId string to flow.ChainID.
func TestConvertChainId(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		chainID string
		valid   bool
	}{
		{"Mainnet", flow.Mainnet.String(), true},
		{"Testnet", flow.Testnet.String(), true},
		{"Emulator", flow.Emulator.String(), true},
		{"Localnet", flow.Localnet.String(), true},
		{"Sandboxnet", flow.Sandboxnet.String(), true},
		{"Previewnet", flow.Previewnet.String(), true},
		{"Benchnet", flow.Benchnet.String(), true},
		{"BftTestnet", flow.BftTestnet.String(), true},
		{"MonotonicEmulator", flow.MonotonicEmulator.String(), true},
		{"Invalid", "invalid-chain", false},
		{"Empty", "", false},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result, err := convert.MessageToChainId(tc.chainID)

			if tc.valid {
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, tc.chainID, result.String())
			} else {
				assert.Error(t, err)
				assert.Nil(t, result)
			}
		})
	}
}
