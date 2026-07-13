package precompiles_test

import (
	"encoding/hex"
	"math/big"
	"testing"

	gethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/evm/precompiles"
)

func TestABIEncodingDecodingFunctions(t *testing.T) {
	t.Parallel()

	t.Run("test address", func(t *testing.T) {
		encodedAddress, err := hex.DecodeString("000000000000000000000000e592427a0aece92de3edee1f18e0157c05861564")
		require.NoError(t, err)
		addr, err := precompiles.ReadAddress(encodedAddress, 0)
		require.NoError(t, err)
		expectedAddress := gethCommon.HexToAddress("e592427a0aece92de3edee1f18e0157c05861564")
		require.Equal(t, expectedAddress, addr)
		reEncoded := make([]byte, precompiles.EncodedAddressSize)
		err = precompiles.EncodeAddress(addr, reEncoded, 0)
		require.NoError(t, err)
		require.Equal(t, encodedAddress, reEncoded)
	})

	t.Run("test boolean", func(t *testing.T) {
		encodedBool, err := hex.DecodeString("0000000000000000000000000000000000000000000000000000000000000001")
		require.NoError(t, err)
		ret, err := precompiles.ReadBool(encodedBool, 0)
		require.NoError(t, err)
		require.True(t, ret)
		reEncoded := make([]byte, precompiles.EncodedBoolSize)
		err = precompiles.EncodeBool(ret, reEncoded, 0)
		require.NoError(t, err)
		require.Equal(t, encodedBool, reEncoded)
	})

	t.Run("test uint64", func(t *testing.T) {
		encodedUint64, err := hex.DecodeString("0000000000000000000000000000000000000000000000000000000000000046")
		require.NoError(t, err)
		ret, err := precompiles.ReadUint64(encodedUint64, 0)
		require.NoError(t, err)
		expectedUint64 := uint64(70)
		require.Equal(t, expectedUint64, ret)
		reEncoded := make([]byte, precompiles.EncodedUint64Size)
		err = precompiles.EncodeUint64(ret, reEncoded, 0)
		require.NoError(t, err)
		require.Equal(t, encodedUint64, reEncoded)

	})

	t.Run("test read uint256", func(t *testing.T) {
		encodedUint256, err := hex.DecodeString("1000000000000000000000000000000000000000000000000000000000000046")
		require.NoError(t, err)
		ret, err := precompiles.ReadUint256(encodedUint256, 0)
		require.NoError(t, err)
		expectedValue, success := new(big.Int).SetString("7237005577332262213973186563042994240829374041602535252466099000494570602566", 10)
		require.True(t, success)
		require.Equal(t, expectedValue, ret)
	})

	t.Run("test fixed size bytes", func(t *testing.T) {
		encodedFixedSizeBytes, err := hex.DecodeString("abcdef1200000000000000000000000000000000000000000000000000000000")
		require.NoError(t, err)
		ret, err := precompiles.ReadBytes4(encodedFixedSizeBytes, 0)
		require.NoError(t, err)
		require.Equal(t, encodedFixedSizeBytes[0:4], ret)

		ret, err = precompiles.ReadBytes8(encodedFixedSizeBytes, 0)
		require.NoError(t, err)
		require.Equal(t, encodedFixedSizeBytes[0:8], ret)

		ret, err = precompiles.ReadBytes32(encodedFixedSizeBytes, 0)
		require.NoError(t, err)
		require.Equal(t, encodedFixedSizeBytes[0:32], ret)

		reEncoded := make([]byte, precompiles.EncodedBytes32Size)
		err = precompiles.EncodeBytes32(ret, reEncoded, 0)
		require.NoError(t, err)
		require.Equal(t, encodedFixedSizeBytes, reEncoded)
	})

	t.Run("test read bytes (variable size)", func(t *testing.T) {
		encodedData, err := hex.DecodeString("0000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000b48656c6c6f20576f726c64000000000000000000000000000000000000000000")
		require.NoError(t, err)

		ret, err := precompiles.ReadBytes(encodedData, 0)
		require.NoError(t, err)
		expectedData, err := hex.DecodeString("48656c6c6f20576f726c64")
		require.NoError(t, err)
		require.Equal(t, expectedData, ret)

		bufferSize := precompiles.SizeNeededForBytesEncoding(expectedData)
		buffer := make([]byte, bufferSize)
		err = precompiles.EncodeBytes(expectedData, buffer, 0, precompiles.EncodedUint64Size)
		require.NoError(t, err)
		require.Equal(t, encodedData, buffer)
	})

	t.Run("test encode bytes (buffer too small)", func(t *testing.T) {
		encodedData, err := hex.DecodeString("e27d73dc3adb81aeea2e5bd35b863fe3f234e1a603fd3bdbaf657c179e67871d")
		require.NoError(t, err)

		bufferSize := precompiles.SizeNeededForBytesEncoding(encodedData)
		require.Equal(t, 3*32, bufferSize)

		buffer := make([]byte, bufferSize-5)
		original := append([]byte(nil), buffer...)
		err = precompiles.EncodeBytes(encodedData, buffer, 0, precompiles.EncodedUint64Size)
		require.Error(t, err)
		require.ErrorContains(t, err, "buffer too small for encoding")
		require.Equal(t, original, buffer)
	})

	t.Run("test encode bytes (multiple chunks)", func(t *testing.T) {
		// The following data needs more than a 32-byte chunk to fit in.
		encodedData, err := hex.DecodeString("e27d73dc3adb81aeea2e5bd35b863fe3f234e1a603fd3bdbaf657c179e67871df1")
		require.NoError(t, err)
		require.Len(t, encodedData, 33)

		bufferSize := precompiles.SizeNeededForBytesEncoding(encodedData)
		require.Equal(t, 4*32, bufferSize)

		buffer := make([]byte, bufferSize)
		err = precompiles.EncodeBytes(encodedData, buffer, 0, precompiles.EncodedUint64Size)
		require.NoError(t, err)

		expectedData, err := hex.DecodeString("00000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000021e27d73dc3adb81aeea2e5bd35b863fe3f234e1a603fd3bdbaf657c179e67871df100000000000000000000000000000000000000000000000000000000000000")
		require.NoError(t, err)
		require.Equal(t, expectedData, buffer)
	})

	t.Run("test size needed for encoding bytes", func(t *testing.T) {
		// len zero
		data := []byte{}
		ret := precompiles.SizeNeededForBytesEncoding(data)
		offsetAndLenEncodingSize := precompiles.EncodedUint64Size + precompiles.EncodedUint64Size
		require.Equal(t, offsetAndLenEncodingSize, ret)

		// data size 1
		data = []byte{1}
		ret = precompiles.SizeNeededForBytesEncoding(data)
		expectedSize := offsetAndLenEncodingSize + precompiles.FixedSizeUnitDataReadSize
		require.Equal(t, expectedSize, ret)

		// data size 32
		data = make([]byte, 32)
		ret = precompiles.SizeNeededForBytesEncoding(data)
		expectedSize = offsetAndLenEncodingSize + precompiles.FixedSizeUnitDataReadSize
		require.Equal(t, expectedSize, ret)

		// data size 33
		data = make([]byte, 33)
		ret = precompiles.SizeNeededForBytesEncoding(data)
		expectedSize = offsetAndLenEncodingSize + precompiles.FixedSizeUnitDataReadSize*2
		require.Equal(t, expectedSize, ret)
	})

}
