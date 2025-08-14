package codec_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/utils/unittest"
)

// roundTripHeaderViaCodec tests encoding and then decoding (AKA round-
// trip) an example message to see if the decoded message matches the
// original encoded message. Why? Both JSON and CBOR require helper
// functions (i.e. MarshalJSON() & MarshalCBOR()) which properly export
// time in the proper format and zone, otherwise running nodes will fail
// due to signature validation failures due to using the incorrectly
// serialized time. When CBOR was first added without the assicated
// helper function then all the unit tests passed but the nodes failed
// as described above. Therefore these functions were added to help the
// next developer who wants to add a new serialization format :-)
func roundTripHeaderViaCodec(t *testing.T, codec network.Codec) {
	block := unittest.BlockFixture()
	proposal := unittest.ProposalFromBlock(block)
	message := messages.Proposal(*proposal)
	encoded, err := codec.Encode(&message)
	assert.NoError(t, err)
	decodedInterface, err := codec.Decode(encoded)
	assert.NoError(t, err)
	decoded := decodedInterface.(*messages.Proposal)
	proposalTrusted, err := flow.NewProposal(flow.UntrustedProposal(*decoded))
	require.NoError(t, err)
	decodedBlock := proposalTrusted.Block
	// compare LastViewTC separately, because it is a pointer field
	if decodedBlock.LastViewTC == nil {
		assert.Equal(t, block.LastViewTC, decodedBlock.LastViewTC)
	} else {
		assert.Equal(t, *block.LastViewTC, *decodedBlock.LastViewTC)
	}
	// compare the rest of the header
	// manually set LastViewTC fields to be equal to pass the Header pointer comparison
	decodedBlock.LastViewTC = block.LastViewTC
	assert.Equal(t, *block.ToHeader(), *decodedBlock.ToHeader())
}

func TestRoundTripHeaderViaCBOR(t *testing.T) {
	codec := unittest.NetworkCodec()
	roundTripHeaderViaCodec(t, codec)
}
