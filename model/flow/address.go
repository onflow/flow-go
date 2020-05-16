// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package flow

import (
	"encoding/binary"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"strings"
)

// Address represents the 8 byte address of an account.
type Address [AddressLength]byte
// AddressState represents the internal state of the address generation mechanism
type AddressState uint64

const (
	// AddressLength is the size of an account address.
	AddressLength = 8
	// AddressStateLength is the size of an account address.
	AddressStateLength = 8
)

func init() {
	gob.Register(Address{})
}

var (
	// ZeroAddress represents the "zero address" (account that no one owns).
	ZeroAddress = nextAddress(AddressState(0))
	// RootAddress represents the root (first) generated account address.
	RootAddress = nextAddress(AddressState(1))
)

// HexToAddress converts a hex string to an Address.
func HexToAddress(h string) Address {
	b, _ := hex.DecodeString(h)
	return BytesToAddress(b)
}

// BytesToAddress returns Address with value b.
//
// If b is larger than 8, b will be cropped from the left.
// If b is smaller than 8, b will be appended by zeroes at the front. 
func BytesToAddress(b []byte) Address {
	var a Address
	if len(b) > AddressLength {
		b = b[len(b)-AddressLength:]
	}
	copy(a[AddressLength-len(b):], b)
	return a
}


// Bytes returns the byte representation of the address.
func (a Address) Bytes() []byte { return a[:] }

// Hex returns the hex string representation of the address.
func (a Address) Hex() string {
	return hex.EncodeToString(a.Bytes())
}

// String returns the string representation of the address.
func (a Address) String() string {
	return a.Hex()
}

// Short returns the string representation of the address with leading zeros
// removed.
func (a Address) Short() string {
	hex := a.String()
	trimmed := strings.TrimLeft(hex, "0")
	if len(trimmed)%2 != 0 {
		trimmed = "0" + trimmed
	}
	return trimmed
}

func (a Address) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("\"%s\"", a.Hex())), nil
}

func (a *Address) UnmarshalJSON(data []byte) error {
	*a = HexToAddress(strings.Trim(string(data), "\""))
	return nil
}


// BytesToAddressState converts an array of bytes into an adress state
func BytesToAddressState(b []byte) AddressState {
	if len(b) > AddressStateLength {
		b = b[len(b)-AddressStateLength:]
	}
	var stateBytes [AddressStateLength]byte
	copy(stateBytes[AddressLength-len(b):], b)
	state := binary.BigEndian.Uint64(stateBytes[:])
	return AddressState(state)
}

// BytesToAddressState converts an array of bytes into an adress state
func (state *AddressState) Bytes() []byte {
	stateBytes := make([]byte, AddressLength)
	binary.BigEndian.PutUint64(stateBytes, uint64(*state))
	return stateBytes
}


// AccountAddress generates a new account address and given an addressing state. 
//
// The second returned value is the new addressing state after the generation.
func AccountAddress(state AddressState) (Address, AddressState) { 
	return nextAddress(state), nextState(state)
}

// returns the new state from an adressing state
func nextState(state AddressState) AddressState{
	newState := state + 1
	return newState
}

// uint64ToAddress returns an address with value v
// The value v fits into the address as the address size is 8
func uint64ToAddress(v uint64) Address {
	var b [AddressLength]byte
	binary.BigEndian.PutUint64(b[:], v)
	return Address(b)
}

// returns the next address from an adressing state
func nextAddress(state AddressState) Address{
	index := uint64(state)
	address := uint64ToAddress(index)
	return address
}