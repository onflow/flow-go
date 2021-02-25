package utils

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

// Bit returns the bit at index `idx` in the byte array `b` (big endian)
//
// The function assumes b has at least idx bits. The caller must make sure this condition is met.
func Bit(b []byte, idx int) int {
	byteValue := int(b[idx>>3])
	idx &= 7
	return (byteValue >> (7 - idx)) & 1
}

// SetBit sets the bit at position i in the byte array b
//
// The function assumes b has at least i bits. The caller must make sure this condition is met.
func SetBit(b []byte, i int) {
	byteIndex := i >> 3
	i &= 7
	b[byteIndex] |= 1 << (7 - i)
}

// MaxUint16 returns the max value of two uint16
func MaxUint16(a, b uint16) uint16 {
	if a > b {
		return a
	}
	return b
}

// Uint16ToBinary converst a uint16 to a byte slice (big endian)
func Uint16ToBinary(integer uint16) []byte {
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, integer)
	return b
}

// Uint64ToBinary converst a uint64 to a byte slice (big endian)
func Uint64ToBinary(integer uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, integer)
	return b
}

// AppendUint8 appends the value byte to the input slice
func AppendUint8(input []byte, value uint8) []byte {
	return append(input, byte(value))
}

// AppendUint16 appends the value bytes to the input slice (big endian)
func AppendUint16(input []byte, value uint16) []byte {
	buffer := make([]byte, 2)
	binary.BigEndian.PutUint16(buffer, value)
	return append(input, buffer...)
}

// AppendUint32 appends the value bytes to the input slice (big endian)
func AppendUint32(input []byte, value uint32) []byte {
	buffer := make([]byte, 4)
	binary.BigEndian.PutUint32(buffer, value)
	return append(input, buffer...)
}

// AppendUint64 appends the value bytes to the input slice (big endian)
func AppendUint64(input []byte, value uint64) []byte {
	buffer := make([]byte, 8)
	binary.BigEndian.PutUint64(buffer, value)
	return append(input, buffer...)
}

// AppendShortData appends data shorter than 16kB
func AppendShortData(input []byte, data []byte) []byte {
	if len(data) > math.MaxUint16 {
		panic(fmt.Sprintf("short data too long! %d", len(data)))
	}
	input = AppendUint16(input, uint16(len(data)))
	input = append(input, data...)
	return input
}

// AppendLongData appends data shorter than 32MB
func AppendLongData(input []byte, data []byte) []byte {
	if len(data) > math.MaxUint32 {
		panic(fmt.Sprintf("long data too long! %d", len(data)))
	}
	input = AppendUint32(input, uint32(len(data)))
	input = append(input, data...)
	return input
}

// ReadSlice reads `size` bytes from the input
func ReadSlice(input []byte, size int) (value []byte, rest []byte, err error) {
	if len(input) < size {
		return nil, input, fmt.Errorf("input size is too small to be splited %d < %d ", len(input), size)
	}
	return input[:size], input[size:], nil
}

// ReadUint8 reads a uint8 from the input and returns the rest
func ReadUint8(input []byte) (value uint8, rest []byte, err error) {
	if len(input) < 1 {
		return 0, input, fmt.Errorf("input size (%d) is too small to read a uint8", len(input))
	}
	return uint8(input[0]), input[1:], nil
}

// ReadUint16 reads a uint16 from the input and returns the rest
func ReadUint16(input []byte) (value uint16, rest []byte, err error) {
	if len(input) < 2 {
		return 0, input, fmt.Errorf("input size (%d) is too small to read a uint16", len(input))
	}
	return binary.BigEndian.Uint16(input[:2]), input[2:], nil
}

// ReadUint32 reads a uint32 from the input and returns the rest
func ReadUint32(input []byte) (value uint32, rest []byte, err error) {
	if len(input) < 4 {
		return 0, input, fmt.Errorf("input size (%d) is too small to read a uint32", len(input))
	}
	return binary.BigEndian.Uint32(input[:4]), input[4:], nil
}

// ReadUint64 reads a uint64 from the input and returns the rest
func ReadUint64(input []byte) (value uint64, rest []byte, err error) {
	if len(input) < 8 {
		return 0, input, fmt.Errorf("input size (%d) is too small to read a uint64", len(input))
	}
	return binary.BigEndian.Uint64(input[:8]), input[8:], nil
}

// ReadShortData read data shorter than 16kB and return the rest of bytes
func ReadShortData(input []byte) (data []byte, rest []byte, err error) {
	var size uint16
	size, rest, err = ReadUint16(input)
	if err != nil {
		return nil, rest, err
	}
	data = rest[:size]
	return
}

// ReadLongData read data shorter than 32MB and return the rest of bytes
func ReadLongData(input []byte) (data []byte, rest []byte, err error) {
	var size uint32
	size, rest, err = ReadUint32(input)
	if err != nil {
		return nil, rest, err
	}
	data = rest[:size]
	return
}

// ReadShortDataFromReader reads data shorter than 16kB from reader
func ReadShortDataFromReader(reader io.Reader) ([]byte, error) {
	buf, err := ReadFromBuffer(reader, 2)
	if err != nil {
		return nil, fmt.Errorf("cannot read short data length: %w", err)
	}

	size, _, err := ReadUint16(buf)
	if err != nil {
		return nil, fmt.Errorf("cannot read short data length: %w", err)
	}

	buf, err = ReadFromBuffer(reader, int(size))
	if err != nil {
		return nil, fmt.Errorf("cannot read short data: %w", err)
	}

	return buf, nil
}

// ReadLongDataFromReader reads data shorter than 16kB from reader
func ReadLongDataFromReader(reader io.Reader) ([]byte, error) {
	buf, err := ReadFromBuffer(reader, 4)
	if err != nil {
		return nil, fmt.Errorf("cannot read long data length: %w", err)
	}
	size, _, err := ReadUint32(buf)
	if err != nil {
		return nil, fmt.Errorf("cannot read long data length: %w", err)
	}
	buf, err = ReadFromBuffer(reader, int(size))
	if err != nil {
		return nil, fmt.Errorf("cannot read long data: %w", err)
	}

	return buf, nil
}

// ReadFromBuffer reads 'length' bytes from the input
func ReadFromBuffer(reader io.Reader, length int) ([]byte, error) {
	if length == 0 {
		return nil, nil
	}
	buf := make([]byte, length)
	_, err := io.ReadFull(reader, buf)
	if err != nil {
		return nil, fmt.Errorf("cannot read data: %w", err)
	}
	return buf, nil
}
