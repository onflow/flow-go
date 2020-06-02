package utils

import "math/rand"

// IsBitSet returns if the bit at position i in the byte array b is set to 1
func IsBitSet(b []byte, i int) bool {
	return b[i/8]&(1<<int(7-i%8)) != 0
}

// SetBit sets the bit at position i in the byte array b to 1
func SetBit(b []byte, i int) {
	b[i/8] |= 1 << int(7-i%8)
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

func GetRandomValues(n int, maxByteSize int) [][]byte {
	values := make([][]byte, 0)
	for i := 0; i < n; i++ {
		byteSize := rand.Intn(maxByteSize)
		value := make([]byte, byteSize)
		rand.Read(value)
		values = append(values, value)
	}
	return values
}
