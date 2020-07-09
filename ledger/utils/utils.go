package utils

import (
	"math/rand"

	"github.com/dapperlabs/flow-go/ledger"
)

// TODO merge these two functions with the one in common
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
// TODO remove this
func GetRandomKeysRandN(maxN int, byteSize int) [][]byte {
	numberOfKeys := rand.Intn(maxN) + 1
	// at least return 1 keys
	if numberOfKeys == 0 {
		numberOfKeys = 1
	}
	return GetRandomKeysFixedN(numberOfKeys, byteSize)
}

// GetRandomKeysFixedN generates n random fixed sized (byteSize) keys
// TODO remove this
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

// TODO remove this
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

// GetRandomPathsRandLen generate m random paths (size: byteSize),
// the number of paths, m, is also randomly selected from the range [1, maxN]
func GetRandomPathsRandLen(maxN int, byteSize int) []ledger.Path {
	numberOfPaths := rand.Intn(maxN) + 1
	return GetRandomPaths(numberOfPaths, byteSize)
}

// GetRandomPaths generates n random (no repetition) fixed sized (byteSize) paths
func GetRandomPaths(n int, byteSize int) []ledger.Path {
	paths := make([]ledger.Path, 0, n)
	alreadySelectPaths := make(map[string]bool)
	i := 0
	for i < n {
		path := make([]byte, byteSize)
		rand.Read(path)
		// deduplicate
		if _, found := alreadySelectPaths[string(path)]; !found {
			paths = append(paths, ledger.Path(path))
			alreadySelectPaths[string(path)] = true
			i++
		}
	}
	return paths
}

// RandomPayload returns a random payload
func RandomPayload(minByteSize int, maxByteSize int) *ledger.Payload {
	keyByteSize := minByteSize + rand.Intn(maxByteSize-minByteSize)
	keydata := make([]byte, keyByteSize)
	rand.Read(keydata)
	key := ledger.Key{KeyParts: []ledger.KeyPart{ledger.KeyPart{Type: 0, Value: keydata}}}
	valueByteSize := minByteSize + rand.Intn(maxByteSize-minByteSize)
	valuedata := make([]byte, valueByteSize)
	rand.Read(valuedata)
	value := ledger.Value(valuedata)
	return &ledger.Payload{Key: key, Value: value}
}

// RandomPayloads returns n random payloads
func RandomPayloads(n int) []ledger.Payload {
	res := make([]ledger.Payload, 0)
	for i := 0; i < n; i++ {
		res = append(res, *RandomPayload(2, 10))
	}
	return res
}
