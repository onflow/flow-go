package packer

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEncodeDecode(t *testing.T) {
	var indices []int
	var count int

	// [0,1,1,1,0,0,0,0] == byte(64+32+16) = byte(112)
	indices = []int{1, 2, 3}
	count = 8
	decoded, err := DecodeSignerIndices(EncodeSignerIndices(indices, count), count)
	require.NoError(t, err)
	require.Equal(t, indices, decoded)

	// [1,1,1,1,1,1,1,1] == byte(255)
	indices = []int{0, 1, 2, 3, 4, 5, 6, 7}
	count = 8
	decoded, err = DecodeSignerIndices(EncodeSignerIndices(indices, count), count)
	require.NoError(t, err)
	require.Equal(t, indices, decoded)

	// no signer
	indices = []int{}
	count = 1
	decoded, err = DecodeSignerIndices(EncodeSignerIndices(indices, count), count)
	require.NoError(t, err)
	require.Equal(t, indices, decoded)

	// two bytes
	indices = []int{1, 2, 3, 4, 5, 6, 9, 12}
	count = 14
	decoded, err = DecodeSignerIndices(EncodeSignerIndices(indices, count), count)
	require.NoError(t, err)
	require.Equal(t, indices, decoded)

	// only signers for first byte
	indices = []int{0, 7}
	count = 14
	decoded, err = DecodeSignerIndices(EncodeSignerIndices(indices, count), count)
	require.NoError(t, err)
	require.Equal(t, indices, decoded)

	// only signers from second byte
	indices = []int{8, 13}
	count = 14
	decoded, err = DecodeSignerIndices(EncodeSignerIndices(indices, count), count)
	require.NoError(t, err)
	require.Equal(t, indices, decoded)
}

func TestDecodeFail(t *testing.T) {
	var indices []int
	var count int
	var err error

	// wrong count: too small
	indices = []int{1, 2, 3}
	_, err = DecodeSignerIndices(EncodeSignerIndices(indices, 8), 3)
	require.Error(t, err)

	// wrong count: too large
	indices = []int{1, 2, 3}
	_, err = DecodeSignerIndices(EncodeSignerIndices(indices, 8), 9)
	require.Error(t, err)

	// wrong indices
	indices = []int{1, 2, 3}
	count = 8
	decoded, err := DecodeSignerIndices(EncodeSignerIndices([]int{1, 2, 4}, count), count)
	require.NoError(t, err)
	require.NotEqual(t, indices, decoded)

	// wrong indices
	indices = []int{1, 2, 3}
	count = 8
	decoded, err = DecodeSignerIndices(EncodeSignerIndices([]int{1, 2}, count), count)
	require.NoError(t, err)
	require.NotEqual(t, indices, decoded)

	// wrong remaining bits
	indices = []int{1, 2, 3}
	count = 4
	badIndices := EncodeSignerIndices(indices, count)
	badIndices[0] ^= byte(1)
	// expect bits to be [0,1,1,1,0,0,0,0], but got [0,1,1,1,0,0,0,1]
	_, err = DecodeSignerIndices(badIndices, count)
	require.Error(t, err)
}
