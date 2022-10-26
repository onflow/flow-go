package cbor_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cborcodec "github.com/onflow/flow-go/model/encoding/cbor"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/network/codec"
	"github.com/onflow/flow-go/network/codec/cbor"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestDecoderDecode(t *testing.T) {
	c := cbor.NewCodec()

	header := unittest.BlockHeaderFixture()
	blockProposal := &messages.BlockProposal{Header: header}

	t.Run("decodes message successfully", func(t *testing.T) {
		var buf bytes.Buffer

		err := c.NewEncoder(&buf).Encode(blockProposal)
		require.NoError(t, err)

		decoded, err := c.NewDecoder(&buf).Decode()
		require.NoError(t, err)
		require.Equal(t, blockProposal, decoded)
	})

	t.Run("returns error when data is empty", func(t *testing.T) {
		var buf bytes.Buffer
		// empty buffer

		decoded, err := c.NewDecoder(&buf).Decode()
		assert.Nil(t, decoded)
		assert.ErrorContains(t, err, "could not decode message")
	})

	t.Run("returns error when data is empty - nil byte", func(t *testing.T) {
		var buf bytes.Buffer

		// nil byte
		buf.WriteByte(0x00)

		decoded, err := c.NewDecoder(&buf).Decode()
		assert.Nil(t, decoded)
		assert.ErrorContains(t, err, "could not decode message")
	})

	t.Run("returns error when data is empty - cbor nil", func(t *testing.T) {
		var buf bytes.Buffer

		// explicit cbor encoding of nil
		err := cborcodec.NewCodec().NewEncoder(&buf).Encode(nil)
		require.NoError(t, err)

		decoded, err := c.NewDecoder(&buf).Decode()
		assert.Nil(t, decoded)
		assert.ErrorIs(t, err, codec.ErrInvalidEncoding)
	})

	t.Run("returns error when data is empty - cbor empty []byte", func(t *testing.T) {
		var buf bytes.Buffer

		// explicit cbor encoding of an empty byte slice
		err := cborcodec.NewCodec().NewEncoder(&buf).Encode([]byte{})
		require.NoError(t, err)

		decoded, err := c.NewDecoder(&buf).Decode()
		assert.Nil(t, decoded)
		assert.ErrorIs(t, err, codec.ErrInvalidEncoding)
	})

	t.Run("returns error when message code is invalid", func(t *testing.T) {
		var buf bytes.Buffer

		// code == min
		err := cborcodec.NewCodec().NewEncoder(&buf).Encode([]byte{codec.CodeMin})
		require.NoError(t, err)

		decoded, err := c.NewDecoder(&buf).Decode()
		assert.Nil(t, decoded)
		assert.True(t, codec.IsErrUnknownMsgCode(err))

		buf.Reset()

		// code == max
		err = cborcodec.NewCodec().NewEncoder(&buf).Encode([]byte{codec.CodeMax})
		require.NoError(t, err)

		decoded, err = c.NewDecoder(&buf).Decode()
		assert.Nil(t, decoded)
		assert.True(t, codec.IsErrUnknownMsgCode(err))

		buf.Reset()

		// code < min
		err = cborcodec.NewCodec().NewEncoder(&buf).Encode([]byte{codec.CodeMin - 1})
		require.NoError(t, err)

		decoded, err = c.NewDecoder(&buf).Decode()
		assert.Nil(t, decoded)
		assert.True(t, codec.IsErrUnknownMsgCode(err))

		buf.Reset()

		// code > max
		err = cborcodec.NewCodec().NewEncoder(&buf).Encode([]byte{codec.CodeMax + 1})
		require.NoError(t, err)

		decoded, err = c.NewDecoder(&buf).Decode()
		assert.Nil(t, decoded)
		assert.True(t, codec.IsErrUnknownMsgCode(err))
	})

	t.Run("returns error when unmarshalling fails - empty", func(t *testing.T) {
		var buf bytes.Buffer

		err := cborcodec.NewCodec().NewEncoder(&buf).Encode([]byte{codec.CodeBlockProposal})
		require.NoError(t, err)

		decoded, err := c.NewDecoder(&buf).Decode()
		assert.Nil(t, decoded)
		assert.True(t, codec.IsErrMsgUnmarshal(err))
	})

	t.Run("returns error when unmarshalling fails - wrong type", func(t *testing.T) {
		// first encode the message to bytes with an incorrect type
		var data bytes.Buffer
		_ = data.WriteByte(codec.CodeCollectionGuarantee)
		encoder := cborcodec.EncMode.NewEncoder(&data)
		err := encoder.Encode(blockProposal)
		require.NoError(t, err)

		// encode the message to bytes
		var buf bytes.Buffer
		encoder = cborcodec.EncMode.NewEncoder(&buf)
		err = encoder.Encode(data.Bytes())
		require.NoError(t, err)

		decoded, err := c.NewDecoder(&buf).Decode()
		assert.Nil(t, decoded)
		assert.True(t, codec.IsErrMsgUnmarshal(err), "incorrect error type: %v", err)
	})

	t.Run("returns error when unmarshalling fails - corrupt", func(t *testing.T) {
		// first encode the message to bytes
		var data bytes.Buffer
		_ = data.WriteByte(codec.CodeBlockProposal)
		encoder := cborcodec.EncMode.NewEncoder(&data)
		err := encoder.Encode(blockProposal)
		require.NoError(t, err)

		var buf bytes.Buffer
		encoder = cborcodec.EncMode.NewEncoder(&buf)
		encoded := data.Bytes()

		// corrupt the encoded message
		encoded[2] = 0x20

		// encode the message to bytes
		err = encoder.Encode(encoded)
		require.NoError(t, err)

		decoded, err := c.NewDecoder(&buf).Decode()
		assert.Nil(t, decoded)
		assert.True(t, codec.IsErrMsgUnmarshal(err), "incorrect error type: %v", err)
	})
}
