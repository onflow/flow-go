package precompiles

import (
	"encoding/binary"
	"errors"
	"math/big"

	gethCommon "github.com/ethereum/go-ethereum/common"
)

// This package provides fast and efficient
// utilities needed for abi encoding and decoding
// encodings are mostly used for testing purpose
// if more complex encoding and decoding is needed please
// use the abi package and pass the ABIs, though
// that has a performance overhead.
const (
	FixedSizeUnitDataReadSize = 32
	Bytes4DataReadSize        = 4
	Bytes8DataReadSize        = 8
	Bytes32DataReadSize       = 32
	Uint64ByteSize            = 8

	EncodedBoolSize    = FixedSizeUnitDataReadSize
	EncodedAddressSize = FixedSizeUnitDataReadSize
	EncodedBytes32Size = FixedSizeUnitDataReadSize
	EncodedBytes4Size  = FixedSizeUnitDataReadSize
	EncodedBytes8Size  = FixedSizeUnitDataReadSize
	EncodedUint64Size  = FixedSizeUnitDataReadSize
	EncodedUint256Size = FixedSizeUnitDataReadSize
)

var ErrInputDataTooSmall = errors.New("input data is too small for decoding")
var ErrBufferTooSmall = errors.New("buffer too small for encoding")
var ErrDataTooLarge = errors.New("input data is too large for encoding")

// ReadAddress reads an address from the buffer at index
func ReadAddress(buffer []byte, index int) (gethCommon.Address, error) {
	if len(buffer) < index+FixedSizeUnitDataReadSize {
		return gethCommon.Address{}, ErrInputDataTooSmall
	}
	paddedData := buffer[index : index+FixedSizeUnitDataReadSize]
	// addresses are zero-padded on the left side.
	addr := gethCommon.BytesToAddress(
		paddedData[FixedSizeUnitDataReadSize-gethCommon.AddressLength:])
	return addr, nil
}

// EncodeAddress encodes the address and add it to the buffer at the index
func EncodeAddress(address gethCommon.Address, buffer []byte, index int) error {
	if len(buffer) < index+EncodedAddressSize {
		return ErrBufferTooSmall
	}
	copy(buffer[index:index+EncodedAddressSize],
		gethCommon.LeftPadBytes(address[:], EncodedAddressSize))
	return nil
}

// ReadBool reads a boolean from the buffer at the index
func ReadBool(buffer []byte, index int) (bool, error) {
	if len(buffer) < index+EncodedBoolSize {
		return false, ErrInputDataTooSmall
	}
	// bools are zero-padded on the left side
	// so we only need to read the last byte
	return uint8(buffer[index+EncodedBoolSize-1]) > 0, nil
}

// EncodeBool encodes a boolean into fixed size unit of encoded data
func EncodeBool(bitSet bool, buffer []byte, index int) error {
	if len(buffer) < index+EncodedBoolSize {
		return ErrBufferTooSmall
	}
	// bit set with left padding
	for i := 0; i < EncodedBoolSize; i++ {
		buffer[index+i] = 0
	}
	if bitSet {
		buffer[index+EncodedBoolSize-1] = 1
	}
	return nil
}

// ReadUint64 reads a uint64 from the buffer at index
func ReadUint64(buffer []byte, index int) (uint64, error) {
	if len(buffer) < index+EncodedUint64Size {
		return 0, ErrInputDataTooSmall
	}
	// data is expected to be big endian (zero-padded on the left side)
	return binary.BigEndian.Uint64(
		buffer[index+EncodedUint64Size-Uint64ByteSize : index+EncodedUint64Size]), nil
}

// EncodeUint64 encodes a uint64 into fixed size unit of encoded data (zero-padded on the left side)
func EncodeUint64(inp uint64, buffer []byte, index int) error {
	if len(buffer) < index+EncodedUint64Size {
		return ErrBufferTooSmall
	}
	encoded := make([]byte, 8)
	binary.BigEndian.PutUint64(encoded, inp)
	copy(buffer[index:index+EncodedUint64Size],
		gethCommon.LeftPadBytes(encoded, EncodedUint64Size),
	)
	return nil
}

// ReadUint256 reads an address from the buffer at index
func ReadUint256(buffer []byte, index int) (*big.Int, error) {
	if len(buffer) < index+EncodedUint256Size {
		return nil, ErrInputDataTooSmall
	}
	// data is expected to be big endian (zero-padded on the left side)
	return new(big.Int).SetBytes(buffer[index : index+EncodedUint256Size]), nil
}

// ReadBytes4 reads a 4 byte slice from the buffer at index
func ReadBytes4(buffer []byte, index int) ([]byte, error) {
	if len(buffer) < index+EncodedBytes4Size {
		return nil, ErrInputDataTooSmall
	}
	// fixed-size byte values are zero-padded on the right side.
	return buffer[index : index+Bytes4DataReadSize], nil
}

// ReadBytes8 reads a 8 byte slice from the buffer at index
func ReadBytes8(buffer []byte, index int) ([]byte, error) {
	if len(buffer) < index+EncodedBytes8Size {
		return nil, ErrInputDataTooSmall
	}
	// fixed-size byte values are zero-padded on the right side.
	return buffer[index : index+Bytes8DataReadSize], nil
}

// ReadBytes32 reads a 32 byte slice from the buffer at index
func ReadBytes32(buffer []byte, index int) ([]byte, error) {
	if len(buffer) < index+Bytes32DataReadSize {
		return nil, ErrInputDataTooSmall
	}
	return buffer[index : index+Bytes32DataReadSize], nil
}

// EncodeBytes32 encodes data into a bytes 32
func EncodeBytes32(data []byte, buffer []byte, index int) error {
	if len(data) > EncodedBytes32Size {
		return ErrDataTooLarge
	}
	if len(buffer) < index+EncodedBytes32Size {
		return ErrBufferTooSmall
	}
	copy(buffer[index:index+EncodedBytes32Size],
		gethCommon.RightPadBytes(data, EncodedBytes32Size),
	)
	return nil
}

// ReadBytes reads a variable length bytes from the buffer
func ReadBytes(buffer []byte, index int) ([]byte, error) {
	if len(buffer) < index+EncodedUint64Size {
		return nil, ErrInputDataTooSmall
	}
	// reading offset (we read into uint64) and adjust index
	offset, err := ReadUint64(buffer, index)
	if err != nil {
		return nil, err
	}
	index = int(offset)
	if len(buffer) < index+EncodedUint64Size {
		return nil, ErrInputDataTooSmall
	}
	// reading length of byte slice
	length, err := ReadUint64(buffer, index)
	if err != nil {
		return nil, err
	}
	index += EncodedUint64Size
	if len(buffer) < index+int(length) {
		return nil, ErrInputDataTooSmall
	}
	return buffer[index : index+int(length)], nil
}

// SizeNeededForBytesEncoding computes the number of bytes needed for bytes encoding
func SizeNeededForBytesEncoding(data []byte) int {
	paddedSize := (len(data) / FixedSizeUnitDataReadSize)
	if len(data)%FixedSizeUnitDataReadSize != 0 {
		paddedSize += FixedSizeUnitDataReadSize
	}
	return EncodedUint64Size + EncodedUint64Size + paddedSize
}

// EncodeBytes encodes the data into the buffer at index and append payload to the
// end of buffer
func EncodeBytes(data []byte, buffer []byte, headerIndex, payloadIndex int) error {
	//// updating offset
	if len(buffer) < headerIndex+EncodedUint64Size {
		return ErrBufferTooSmall
	}
	dataSize := len(data)
	// compute padded data size
	paddedSize := (dataSize / FixedSizeUnitDataReadSize)
	if dataSize%FixedSizeUnitDataReadSize != 0 {
		paddedSize += FixedSizeUnitDataReadSize
	}
	if len(buffer) < payloadIndex+EncodedUint64Size+paddedSize {
		return ErrBufferTooSmall
	}

	err := EncodeUint64(uint64(payloadIndex), buffer, headerIndex)
	if err != nil {
		return err
	}
	headerIndex += EncodedUint64Size

	//// updating payload
	// padding data
	if dataSize%FixedSizeUnitDataReadSize != 0 {
		data = gethCommon.RightPadBytes(data, paddedSize)
	}

	// adding length
	err = EncodeUint64(uint64(dataSize), buffer, payloadIndex)
	if err != nil {
		return err
	}
	payloadIndex += EncodedUint64Size
	// adding data
	copy(buffer[payloadIndex:payloadIndex+len(data)], data)
	return nil
}
