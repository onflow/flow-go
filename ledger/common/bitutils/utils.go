package bitutils

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
func SetBit(b []byte, idx int, value int) {
	byteIndex := idx >> 3
	idx &= 7
	mask := byte(1 << (7 - idx))
	if value == 0 {
		b[byteIndex] &= ^mask
	} else {
		b[byteIndex] |= mask
	}
}

// MakeBitVector allocates a byte slice of minimal size that can hold numberBits.
func MakeBitVector(numberBits int) []byte {
	return make([]byte, (numberBits+7)>>3)
}
