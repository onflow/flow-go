package common

import (
	"fmt"
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

// MaxUint16 returns the max value of two uint16
func MaxUint16(a, b uint16) uint16 {
	if a > b {
		return a
	}
	return b
}
