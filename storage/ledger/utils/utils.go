package utils

import (
	"math/rand"

	"github.com/dapperlabs/flow-go/model/flow"
)

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

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func RandomString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

func GetRandomRegisterIDs(n int) []flow.RegisterID {

	registers := make([]flow.RegisterID, n)

	for i := 0; i < n; i++ {
		registers[i].Controller = RandomString(10)
		registers[i].Owner = RandomString(10)
		registers[i].Key = RandomString(10)
	}

	return registers
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
