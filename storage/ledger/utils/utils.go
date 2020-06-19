package utils

import "math/rand"

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

// GetBits converts a byte slice into slice of int (0 or 1)
func GetBits(bs []byte) []int {
	r := make([]int, len(bs)*8)
	for i, b := range bs {
		for j := 0; j < 8; j++ {
			r[i*8+j] = int(b >> uint(7-j) & 0x01)
		}
	}
	return r
}

// GetRandomKeysRandN generate m random keys (size: byteSize),
// assuming m is also randomly selected from zero to maxN
func GetRandomKeysRandN(maxN int, byteSize int) [][]byte {
	numberOfKeys := rand.Intn(maxN) + 1
	// at least return 1 keys
	if numberOfKeys == 0 {
		numberOfKeys = 1
	}
	return GetRandomKeysFixedN(numberOfKeys, byteSize)
}

// GetRandomKeysFixedN generates n random fixed sized (byteSize) keys
func GetRandomKeysFixedN(n int, byteSize int) [][]byte {
	keys := make([][]byte, 0)
	alreadySelectKeys := make(map[string]bool)
	i := 0
	for i < n {
		key := make([]byte, byteSize)
		rand.Read(key)
		// deduplicate
		if _, found := alreadySelectKeys[string(key)]; !found {
			keys = append(keys, key)
			alreadySelectKeys[string(key)] = true
			i++
		}
	}
	return keys
}

func GetRandomValues(n int, minByteSize, maxByteSize int) [][]byte {
	if minByteSize > maxByteSize {
		panic("minByteSize cannot be smaller then maxByteSize")
	}
	values := make([][]byte, 0)
	for i := 0; i < n; i++ {
		var byteSize = maxByteSize
		if minByteSize < maxByteSize {
			byteSize = minByteSize + rand.Intn(maxByteSize-minByteSize)
		}
		value := make([]byte, byteSize)
		rand.Read(value)
		values = append(values, value)
	}
	return values
}
