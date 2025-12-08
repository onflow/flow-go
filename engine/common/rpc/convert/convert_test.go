package convert_test

import (
	"fmt"
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
	assert.Contains(t, err.Error(), "invalid state commitment length")
}

// TestConvertAggregatedSignatures tests converting a slice of flow.AggregatedSignatures to and from
// protobuf messages.
func TestConvertAggregatedSignatures(t *testing.T) {
	t.Parallel()

	aggSigs := unittest.Seal.AggregatedSignatureFixtures(2)

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

	t.Run("valid chain IDs", func(t *testing.T) {
		t.Parallel()

		validChainIDs := []flow.ChainID{
			flow.Mainnet,
			flow.Testnet,
			flow.Emulator,
			flow.Localnet,
			flow.Sandboxnet,
			flow.Previewnet,
			flow.Benchnet,
			flow.BftTestnet,
			flow.MonotonicEmulator,
		}

		for _, chainID := range validChainIDs {
			t.Run(chainID.String(), func(t *testing.T) {
				t.Parallel()

				result, err := convert.MessageToChainId(chainID.String())
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, chainID, *result)
			})
		}
	})

	t.Run("invalid chain IDs", func(t *testing.T) {
		t.Parallel()

		invalid := []string{"invalid-chain", ""}

		for _, chainID := range invalid {
			t.Run(fmt.Sprintf("invalid_%q", chainID), func(t *testing.T) {
				t.Parallel()

				result, err := convert.MessageToChainId(chainID)
				assert.Error(t, err)
				assert.Nil(t, result)
				assert.Contains(t, err.Error(), "invalid chainId")
			})
		}
	})
}
