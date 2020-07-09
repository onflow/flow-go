package common

import (
	"fmt"

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
func SplitByPath(paths []ledger.Path, payloads []ledger.Payload, bitIndex int) ([]ledger.Path, []ledger.Payload, []ledger.Path, []ledger.Payload, error) {
	rpaths := make([]ledger.Path, 0, len(paths))
	rpayloads := make([]ledger.Payload, 0, len(payloads))
	lpaths := make([]ledger.Path, 0, len(paths))
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
func SplitSortedPaths(paths []ledger.Path, bitIndex int) ([]ledger.Path, []ledger.Path, error) {
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

// MaxUint16 returns the max value of two uint16
func MaxUint16(a, b uint16) uint16 {
	if a > b {
		return a
	}
	return b
}
