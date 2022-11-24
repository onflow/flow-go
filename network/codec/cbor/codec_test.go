package cbor_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/network/codec"
	"github.com/onflow/flow-go/network/codec/cbor"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestCodec_Decode(t *testing.T) {
	t.Parallel()

	c := cbor.NewCodec()

	t.Run("decodes message successfully", func(t *testing.T) {
		t.Parallel()

		header := unittest.BlockHeaderFixture()
		data := &messages.BlockProposal{Header: header}

		encoded, err := c.Encode(data)
		require.NoError(t, err)

		decoded, err := c.Decode(encoded)
		require.NoError(t, err)
		require.Equal(t, data, decoded)
	})

	t.Run("returns error when data is empty", func(t *testing.T) {
		t.Parallel()

		decoded, err := c.Decode(nil)
		assert.Nil(t, decoded)
		assert.True(t, codec.IsErrInvalidEncoding(err))

		decoded, err = c.Decode([]byte{})
		assert.Nil(t, decoded)
		assert.True(t, codec.IsErrInvalidEncoding(err))
	})

	t.Run("returns error when message code is invalid", func(t *testing.T) {
		t.Parallel()

		decoded, err := c.Decode([]byte{codec.CodeMin})
		assert.Nil(t, decoded)
		assert.True(t, codec.IsErrUnknownMsgCode(err))

		decoded, err = c.Decode([]byte{codec.CodeMax})
		assert.Nil(t, decoded)
		assert.True(t, codec.IsErrUnknownMsgCode(err))

		decoded, err = c.Decode([]byte{codec.CodeMin - 1})
		assert.Nil(t, decoded)
		assert.True(t, codec.IsErrUnknownMsgCode(err))

		decoded, err = c.Decode([]byte{codec.CodeMax + 1})
		assert.Nil(t, decoded)
		assert.True(t, codec.IsErrUnknownMsgCode(err))
	})

	t.Run("returns error when unmarshalling fails - empty", func(t *testing.T) {
		t.Parallel()

		decoded, err := c.Decode([]byte{codec.CodeBlockProposal})
		assert.Nil(t, decoded)
		assert.True(t, codec.IsErrMsgUnmarshal(err))
	})

	t.Run("returns error when unmarshalling fails - wrong type", func(t *testing.T) {
		t.Parallel()

		header := unittest.BlockHeaderFixture()
		data := &messages.BlockProposal{Header: header}

		encoded, err := c.Encode(data)
		require.NoError(t, err)

		encoded[0] = codec.CodeCollectionGuarantee

		decoded, err := c.Decode(encoded)
		assert.Nil(t, decoded)
		assert.True(t, codec.IsErrMsgUnmarshal(err))
	})

	t.Run("returns error when unmarshalling fails - corrupt", func(t *testing.T) {
		t.Parallel()

		header := unittest.BlockHeaderFixture()
		data := &messages.BlockProposal{Header: header}

		encoded, err := c.Encode(data)
		require.NoError(t, err)

		encoded[2] = 0x20 // corrupt payload

		decoded, err := c.Decode(encoded)
		assert.Nil(t, decoded)
		assert.True(t, codec.IsErrMsgUnmarshal(err))
	})
}
