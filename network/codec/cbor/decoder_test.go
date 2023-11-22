package cbor_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cborcodec "github.com/onflow/flow-go/model/encoding/cbor"
	"github.com/onflow/flow-go/network/codec"
	"github.com/onflow/flow-go/network/codec/cbor"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestDecoder_Decode(t *testing.T) {
	c := cbor.NewCodec()

	blockProposal := unittest.ProposalFixture()

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
		assert.True(t, codec.IsErrInvalidEncoding(err))
	})

	t.Run("returns error when data is empty - nil byte", func(t *testing.T) {
		var buf bytes.Buffer

		// nil byte
		buf.WriteByte(0x00)

		decoded, err := c.NewDecoder(&buf).Decode()
		assert.Nil(t, decoded)
		assert.True(t, codec.IsErrInvalidEncoding(err))
	})

	t.Run("returns error when data is empty - cbor nil", func(t *testing.T) {
		var buf bytes.Buffer

		// explicit cbor encoding of nil
		err := cborcodec.NewCodec().NewEncoder(&buf).Encode(nil)
		require.NoError(t, err)

		decoded, err := c.NewDecoder(&buf).Decode()
		assert.Nil(t, decoded)
		assert.True(t, codec.IsErrInvalidEncoding(err))
	})

	t.Run("returns error when data is empty - cbor empty []byte", func(t *testing.T) {
		var buf bytes.Buffer

		// explicit cbor encoding of an empty byte slice
		err := cborcodec.NewCodec().NewEncoder(&buf).Encode([]byte{})
		require.NoError(t, err)

		decoded, err := c.NewDecoder(&buf).Decode()
		assert.Nil(t, decoded)
		assert.True(t, codec.IsErrInvalidEncoding(err))
	})

	t.Run("returns error when message code is invalid", func(t *testing.T) {
		var buf bytes.Buffer

		// the first byte is the message code, the remaining bytes are the message
		// in this test, only the first byte is set since there should be an error after reading
		// the invalid code
		datas := [][]byte{
			{codec.CodeMin.Uint8() - 1}, // code < min
			{codec.CodeMin.Uint8()},     // code == min
			{codec.CodeMax.Uint8()},     // code == max
			{codec.CodeMax.Uint8() + 1}, // code > max
		}

		for i := range datas {
			err := cborcodec.NewCodec().NewEncoder(&buf).Encode(datas[i])
			require.NoError(t, err)

			decoded, err := c.NewDecoder(&buf).Decode()
			assert.Nil(t, decoded)
			assert.True(t, codec.IsErrUnknownMsgCode(err))

			buf.Reset()
		}
	})

	t.Run("returns error when unmarshalling fails - empty", func(t *testing.T) {
		var buf bytes.Buffer

		err := cborcodec.NewCodec().NewEncoder(&buf).Encode([]byte{codec.CodeBlockProposal.Uint8()})
		require.NoError(t, err)

		decoded, err := c.NewDecoder(&buf).Decode()
		assert.Nil(t, decoded)
		assert.True(t, codec.IsErrMsgUnmarshal(err))
	})

	t.Run("returns error when unmarshalling fails - wrong type", func(t *testing.T) {
		// first encode the message to bytes with an incorrect type
		var data bytes.Buffer
		_ = data.WriteByte(codec.CodeCollectionGuarantee.Uint8())
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
		_ = data.WriteByte(codec.CodeBlockProposal.Uint8())
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
