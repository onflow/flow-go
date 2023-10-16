package p2pbuilder

import (
	"testing"

	"github.com/pbnjay/memory"
	"github.com/stretchr/testify/require"
)

func TestAllowedMemoryScale(t *testing.T) {
	m := memory.TotalMemory()
	require.True(t, m > 0)

	// scaling with factor of 1 should return the total memory.
	s, err := AllowedMemory(1)
	require.NoError(t, err)
	require.Equal(t, int64(m), s)

	// scaling with factor of 0 should return an error.
	_, err = AllowedMemory(0)
	require.Error(t, err)

	// scaling with factor of -1 should return an error.
	_, err = AllowedMemory(-1)
	require.Error(t, err)

	// scaling with factor of 2 should return an error.
	_, err = AllowedMemory(2)
	require.Error(t, err)

	// scaling with factor of 0.5 should return half the total memory.
	s, err = AllowedMemory(0.5)
	require.NoError(t, err)
	require.Equal(t, int64(m/2), s)

	// scaling with factor of 0.1 should return 10% of the total memory.
	s, err = AllowedMemory(0.1)
	require.NoError(t, err)
	require.Equal(t, int64(m/10), s)

	// scaling with factor of 0.01 should return 1% of the total memory.
	s, err = AllowedMemory(0.01)
	require.NoError(t, err)
	require.Equal(t, int64(m/100), s)

	// scaling with factor of 0.001 should return 0.1% of the total memory.
	s, err = AllowedMemory(0.001)
	require.NoError(t, err)
	require.Equal(t, int64(m/1000), s)

	// scaling with factor of 0.0001 should return 0.01% of the total memory.
	s, err = AllowedMemory(0.0001)
	require.NoError(t, err)
	require.Equal(t, int64(m/10000), s)
}

func TestAllowedFileDescriptorsScale(t *testing.T) {
	// getting actual file descriptor limit.
	fd, err := getNumFDs()
	require.NoError(t, err)
	require.True(t, fd > 0)

	// scaling with factor of 1 should return the total file descriptors.
	s, err := AllowedFileDescriptors(1)
	require.NoError(t, err)
	require.Equal(t, fd, s)

	// scaling with factor of 0 should return an error.
	_, err = AllowedFileDescriptors(0)
	require.Error(t, err)

	// scaling with factor of -1 should return an error.
	_, err = AllowedFileDescriptors(-1)
	require.Error(t, err)

	// scaling with factor of 2 should return an error.
	_, err = AllowedFileDescriptors(2)
	require.Error(t, err)

	// scaling with factor of 0.5 should return half the total file descriptors.
	s, err = AllowedFileDescriptors(0.5)
	require.NoError(t, err)
	require.Equal(t, fd/2, s)

	// scaling with factor of 0.1 should return 10% of the total file descriptors.
	s, err = AllowedFileDescriptors(0.1)
	require.NoError(t, err)
	require.Equal(t, fd/10, s)

	// scaling with factor of 0.01 should return 1% of the total file descriptors.
	s, err = AllowedFileDescriptors(0.01)
	require.NoError(t, err)
	require.Equal(t, fd/100, s)

	// scaling with factor of 0.001 should return 0.1% of the total file descriptors.
	s, err = AllowedFileDescriptors(0.001)
	require.NoError(t, err)
	require.Equal(t, fd/1000, s)

	// scaling with factor of 0.0001 should return 0.01% of the total file descriptors.
	s, err = AllowedFileDescriptors(0.0001)
	require.NoError(t, err)
	require.Equal(t, fd/10000, s)
}
