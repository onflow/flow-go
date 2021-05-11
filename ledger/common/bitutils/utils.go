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
func SetBit(b []byte, i int) {
	byteIndex := i >> 3
	i &= 7
	b[byteIndex] |= 1 << (7 - i)
}
