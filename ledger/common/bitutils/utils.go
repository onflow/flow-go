package bitutils

// ReadBit returns the bit at index `idx` in the byte array `b` (big endian)
// The function panics, if the byte slice is too short.
func ReadBit(b []byte, idx int) int {
	byteValue := int(b[idx>>3])
	idx &= 7
	return (byteValue >> (7 - idx)) & 1
}

// WriteBit assigns value `v` to the bit at index `i` in the byte array `b`.
// The function panics, if the byte slice is too short. We follow the common
// convention of converting between integer and boolean/bit values:
//  * int value == 0   <=>   false   <=>   bit 0
//  * int value != 0   <=>   true    <=>   bit 1
func WriteBit(b []byte, i int, value int) {
	if value == 0 {
		ClearBit(b, i)
	} else {
		SetBit(b, i)
	}
}

// SetBit sets the bit at index `i` in the byte array `b`, i.e. it assigns
// value 1 to the bit. The function panics, if the byte slice is too short.
func SetBit(b []byte, i int) {
	byteIndex := i >> 3
	i &= 7
	mask := byte(1 << (7 - i))
	b[byteIndex] |= mask
}

// ClearBit clears the bit at index `i` in the byte slice `b`, i.e. it assigns
// value 0 to the bit. The function panics, if the byte slice is too short.
func ClearBit(b []byte, i int) {
	byteIndex := i >> 3
	i &= 7
	mask := byte(1 << (7 - i))
	b[byteIndex] &= ^mask
}

// MakeBitVector allocates a byte slice of minimal size that can hold numberBits.
func MakeBitVector(numberBits int) []byte {
	return make([]byte, (numberBits+7)>>3)
}
