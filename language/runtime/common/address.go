package common

import (
	"fmt"
)

const AddressLength = 20

type Address [AddressLength]byte

// BytesToAddress returns Address with value b.
//
// If b is larger than len(h), b will be cropped from the left.
func BytesToAddress(b []byte) Address {
	var a Address
	a.SetBytes(b)
	return a
}

// Hex returns the hex string representation of the address.
func (a Address) Hex() string {
	return fmt.Sprintf("%x", a[:])
}

func (a Address) String() string {
	return a.Hex()
}

// SetBytes sets the address to the value of b.
//
// If b is larger than len(a) it will panic.
func (a *Address) SetBytes(b []byte) {
	if len(b) > len(a) {
		b = b[len(b)-AddressLength:]
	}

	copy(a[AddressLength-len(b):], b)
}
