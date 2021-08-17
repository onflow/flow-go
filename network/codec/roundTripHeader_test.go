package codec_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/network"
	cborcodec "github.com/onflow/flow-go/network/codec/cbor"
	jsoncodec "github.com/onflow/flow-go/network/codec/json"
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
	message := &messages.BlockProposal{Header: block.Header, Payload: block.Payload}
	encoded, err := codec.Encode(message)
	assert.NoError(t, err)
	decodedInterface, err := codec.Decode(encoded)
	assert.NoError(t, err)
	decoded := decodedInterface.(*messages.BlockProposal)
	assert.Equal(t, message.Header.ProposerSigData, decoded.Header.ProposerSigData)
	messageHeader := fmt.Sprintf("- .Header=%+v\n", message.Header)
	decodedHeader := fmt.Sprintf("- .Header=%+v\n", decoded.Header)
	assert.Equal(t, messageHeader, decodedHeader)
}

func TestRoundTripHeaderViaJSON(t *testing.T) {
	codec := jsoncodec.NewCodec()
	roundTripHeaderViaCodec(t, codec)
}

func TestRoundTripHeaderViaCBOR(t *testing.T) {
	codec := cborcodec.NewCodec()
	roundTripHeaderViaCodec(t, codec)
}
