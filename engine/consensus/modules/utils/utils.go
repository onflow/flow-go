package utils

import "encoding/binary"

func ConvertBytesToHash(data []byte) [32]byte {
	var hashArr [32]byte

	if len(data) >= 32 {
		copy(hashArr[:], data[:32])
	}

	return hashArr
}

func ConvertUintToByte(num uint64) []byte {
	// convert uint64 to []byte
	buf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(buf, num)
	b := buf[:n]

	return b
}

func ConvertByteArrToUint(hash []byte) uint64 {
	// convert []byte to int64
	x, _ := binary.Uvarint(hash)
	return x
}

func CheckError(e error) {
	if e != nil {
		panic(e)
	}
}

func ConvertBoolToByte(b bool) byte {
	if b == true {
		return byte(1)
	}
	return byte(0)
}

func ConvertBoolSliceToByteSlice(bools []bool) []byte {
	bytes := make([]byte, len(bools))
	for i, val := range bools {
		bytes[i] = ConvertBoolToByte(val)
	}

	return bytes
}
