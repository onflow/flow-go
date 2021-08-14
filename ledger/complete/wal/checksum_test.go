package wal

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_ChecksumWriter(t *testing.T) {

	buffer := &bytes.Buffer{}

	writer := NewCRC32Writer(buffer)

	_, err := writer.Write([]byte("a"))
	require.NoError(t, err)

	_, err = writer.Write([]byte("b"))
	require.NoError(t, err)

	_, err = writer.Write([]byte("c"))
	require.NoError(t, err)

	crc32 := writer.Crc32()

	require.Equal(t, uint32(0x364b3fb7), crc32)
	require.Equal(t, "abc", buffer.String())
}

func Test_ChecksumReader(t *testing.T) {

	buffer := bytes.NewBufferString("abc")

	reader := NewCRC32Reader(buffer)

	b := make([]byte, 1)

	_, err := reader.Read(b)
	require.NoError(t, err)

	_, err = reader.Read(b)
	require.NoError(t, err)

	_, err = reader.Read(b)
	require.NoError(t, err)

	crc32 := reader.Crc32()

	require.Equal(t, uint32(0x364b3fb7), crc32)
	require.Equal(t, 0, buffer.Len())
}
