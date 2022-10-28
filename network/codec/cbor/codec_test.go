package cbor_test

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model"
	"github.com/onflow/flow-go/model/messages"
	pkgcodec "github.com/onflow/flow-go/network/codec"
	"github.com/onflow/flow-go/network/codec/cbor"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestEncodeDecode_Nil ensures that, if the codec successfully decodes a message,
// the return value is not nil.
func TestEncodeDecode_Nil(t *testing.T) {
	codec := cbor.NewCodec()
	msg := (*messages.BlockProposal)(nil)
	encoded, err := codec.Encode(msg)
	require.NoError(t, err)
	decoded, err := codec.Decode(encoded)
	require.NoError(t, err)
	require.NotNil(t, decoded)
}

// TestEncodeDecode_StructureValid ensures that the codec enforces structural validity
// rules for a successfully decoded message, when the underlying Go type implements
// model.StructureValidator.
func TestEncodeDecode_StructureValid(t *testing.T) {
	codec := cbor.NewCodec()

	// message with valid structure should be decoded as usual
	t.Run("valid structure", func(t *testing.T) {
		msg := &messages.BlockResponse{
			Nonce:  rand.Uint64(),
			Blocks: unittest.BlockFixtures(5),
		}
		// sanity check - ensure message type is a model.StructureValidator and passes validation
		var _ model.StructureValidator = msg
		require.NoError(t, msg.StructureValid())

		encoded, err := codec.Encode(msg)
		require.NoError(t, err)
		decoded, err := codec.Decode(encoded)
		require.NoError(t, err)
		require.Equal(t, msg, decoded)
	})
	// otherwise decodable message with invalid structure should return codec.ErrMsgUnmarshal
	t.Run("invalid structure", func(t *testing.T) {
		msg := &messages.BlockResponse{
			Nonce:  rand.Uint64(),
			Blocks: unittest.BlockFixtures(5),
		}
		msg.Blocks[3].Payload = nil // a nil required field is an invalid structure

		// sanity check - ensure message type is a model.StructureValidator and fails validation
		var _ model.StructureValidator = msg
		err := msg.StructureValid()
		require.Error(t, err)
		require.True(t, model.IsStructureInvalidError(err))

		encoded, err := codec.Encode(msg)
		require.NoError(t, err)
		_, err = codec.Decode(encoded)
		require.Error(t, err)
		require.True(t, pkgcodec.IsErrMsgUnmarshal(err))
	})
}
