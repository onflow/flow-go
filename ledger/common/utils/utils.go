package utils

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

// IsBitSet returns if the bit at index `idx` in the byte array `b` is set to 1 (big endian)
// TODO: remove error return
func IsBitSet(b []byte, idx int) (bool, error) {
	if idx >= len(b)*8 {
		return false, fmt.Errorf("input (%v) only has %d bits, can't look up bit %d", b, len(b)*8, idx)
	}
	return b[idx/8]&(1<<int(7-idx%8)) != 0, nil
}

// SetBit sets the bit at position i in the byte array b to 1
// TODO: remove error return
func SetBit(b []byte, i int) error {
	if i >= len(b)*8 {
		return fmt.Errorf("input (%v) only has %d bits, can't set bit %d", b, len(b)*8, i)
	}
	b[i/8] |= 1 << int(7-i%8)
	return nil
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

func AppendUint8(input []byte, value uint8) []byte {
	return append(input, byte(value))
}

func AppendUint16(input []byte, value uint16) []byte {
	buffer := make([]byte, 2)
	binary.BigEndian.PutUint16(buffer, value)
	return append(input, buffer...)
}

func AppendUint32(input []byte, value uint32) []byte {
	buffer := make([]byte, 4)
	binary.BigEndian.PutUint32(buffer, value)
	return append(input, buffer...)
}

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

func ReadSlice(input []byte, size int) (value []byte, rest []byte, err error) {
	if len(input) < size {
		return nil, input, fmt.Errorf("input size is too small to be splited %d < %d ", len(input), size)
	}
	return input[:size], input[size:], nil
}

func ReadUint8(input []byte) (value uint8, rest []byte, err error) {
	if len(input) < 1 {
		return 0, input, fmt.Errorf("input size (%d) is too small to read a uint8", len(input))
	}
	return uint8(input[0]), input[1:], nil
}

func ReadUint16(input []byte) (value uint16, rest []byte, err error) {
	if len(input) < 2 {
		return 0, input, fmt.Errorf("input size (%d) is too small to read a uint16", len(input))
	}
	return binary.BigEndian.Uint16(input[:2]), input[2:], nil
}

func ReadUint32(input []byte) (value uint32, rest []byte, err error) {
	if len(input) < 4 {
		return 0, input, fmt.Errorf("input size (%d) is too small to read a uint32", len(input))
	}
	return binary.BigEndian.Uint32(input[:4]), input[4:], nil
}

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
