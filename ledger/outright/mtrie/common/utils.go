package common

import (
	"fmt"
	"math/rand"

	"github.com/dapperlabs/flow-go/ledger"
)

// IsBitSet returns if the bit at index `idx` in the byte array `b` is set to 1 (big endian)
// TODO: remove error return
func IsBitSet(b []byte, idx int) (bool, error) {
	if idx >= len(b)*8 {
		return false, fmt.Errorf("input (%v) only has %d bits, can't look up bit %d", b, len(b)*8, idx)
	}
	return b[idx/8]&(1<<int(7-idx%8)) != 0, nil
}

// SetBit sets the bit at position i in the byte array b to 1
// TODO: remove error return
func SetBit(b []byte, i int) error {
	if i >= len(b)*8 {
		return fmt.Errorf("input (%v) only has %d bits, can't set bit %d", b, len(b)*8, i)
	}
	b[i/8] |= 1 << int(7-i%8)
	return nil
}

// SplitByPath splits an slice of payloads based on the value of bit (bitIndex) of paths
// TODO: remove error return
func SplitByPath(paths [][]byte, payloads []ledger.Payload, bitIndex int) ([][]byte, []ledger.Payload, [][]byte, []ledger.Payload, error) {
	rpaths := make([][]byte, 0, len(paths))
	rpayloads := make([]ledger.Payload, 0, len(payloads))
	lpaths := make([][]byte, 0, len(paths))
	lpayloads := make([]ledger.Payload, 0, len(payloads))

	for i, path := range paths {
		bitIsSet, err := IsBitSet(path, bitIndex)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("can't split payloads, error: %v", err)
		}
		if bitIsSet {
			rpaths = append(rpaths, path)
			rpayloads = append(rpayloads, payloads[i])
		} else {
			lpaths = append(lpaths, path)
			lpayloads = append(lpayloads, payloads[i])
		}
	}
	return lpaths, lpayloads, rpaths, rpayloads, nil
}

// SplitSortedPaths splits a set of ordered paths based on the value of bit (bitIndex)
func SplitSortedPaths(paths [][]byte, bitIndex int) ([][]byte, [][]byte, error) {
	for i, path := range paths {
		bitIsSet, err := IsBitSet(path, bitIndex)
		if err != nil {
			return nil, nil, fmt.Errorf("can't split paths, error: %v", err)
		}
		// found the breaking point
		if bitIsSet {
			return paths[:i], paths[i:], nil
		}
	}
	// all paths have unset bit at bitIndex
	return paths, nil, nil
}

// GetRandomPathsRandN generate m random paths (size: byteSize),
// the number of paths, m, is also randomly selected from the range [1, maxN]
func GetRandomPathsRandN(maxN int, byteSize int) [][]byte {
	numberOfPaths := rand.Intn(maxN) + 1
	return GetRandomPathsFixedN(numberOfPaths, byteSize)
}

// GetRandomPathsFixedN generates n random (no repetition) fixed sized (byteSize) paths
func GetRandomPathsFixedN(n int, byteSize int) [][]byte {
	paths := make([][]byte, 0, n)
	alreadySelectPaths := make(map[string]bool)
	i := 0
	for i < n {
		path := make([]byte, byteSize)
		rand.Read(path)
		// deduplicate
		if _, found := alreadySelectPaths[string(path)]; !found {
			paths = append(paths, path)
			alreadySelectPaths[string(path)] = true
			i++
		}
	}
	return paths
}

// GetRandomByteSlices generate an slice of n
// random byte slices of random size from the range [1, maxByteSize]
func GetRandomByteSlices(n int, maxByteSize int) [][]byte {
	values := make([][]byte, 0, n)
	for i := 0; i < n; i++ {
		byteSize := rand.Intn(maxByteSize) + 1
		value := make([]byte, byteSize)
		rand.Read(value)
		values = append(values, value)
	}
	return values
}

// MaxUint16 returns the max value of two uint16
func MaxUint16(a, b uint16) uint16 {
	if a > b {
		return a
	}
	return b
}
