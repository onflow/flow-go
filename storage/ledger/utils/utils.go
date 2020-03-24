package utils

// splitKeys splits the byte array containing keys based on whose bit at position i is set to 1
// it returns the left and right sides of the byte array and the index at which they were split
// This assumes that keys is sorted and each key in keys is big endian
func SplitKeys(keys [][]byte, height int) ([][]byte, [][]byte, int) {
	// create byte array with bit at position height set to 1
	split := make([]byte, len(keys[0]))
	SetBit(split, height)
	// splits keys at the smallest index i where keys[i] >= split
	for i, key := range keys {
		if IsBitSet(key, height) {
			return keys[:i], keys[i:], i
		}
	}

	return keys, nil, len(keys)
}

// IsBitSet returns if the bit at position i in the byte array b is set to 1
func IsBitSet(b []byte, i int) bool {
	return b[i/8]&(1<<int(7-i%8)) != 0
}

// SetBit sets the bit at position i in the byte array b to 1
func SetBit(b []byte, i int) {
	b[i/8] |= 1 << int(7-i%8)
}

func GetBits(bs []byte) []int {
	r := make([]int, len(bs)*8)
	for i, b := range bs {
		for j := 0; j < 8; j++ {
			r[i*8+j] = int(b >> uint(7-j) & 0x01)
		}
	}
	return r
}
