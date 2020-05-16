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

const (
	// [n,k,d]-Linear code parameters 
	// The linear code used in the account addressing is a [64,45,7]
	// It generates a [64,45]-code, which is the space of Flow account addresses
	//
	// size of the code words in bits,
	// which is also the size of the account addresses in bits
	n = AddressLength << 3
	// size of the words in bits.
	// 2^k is the total number of possible account addresses.
	k = 45
	// number of code parity bits
	p = n-k
	// distance of the linear code.
	// d is the minimum hamming distance between any two Flow account addresses.
	// d is also the minimum hamming weight of all non-zero account addresses.
	d = 7

	// the maximum value of the internal state, 2^k.
	maxState = (1 << k) - 1
)

// AccountAddress generates a new account address given an addressing state. 
//
// The second returned value is the new updated addressing state.
// Each state is mapped to exactly one address. There are as many addresses 
// as states.
// ZeroAddress corresponds to the state "0" while RootAddress corresponds to the 
// state "1".
func AccountAddress(state AddressState) (Address, AddressState, error) { 
	newState, err := nextState(state)
	if err != nil {
		return Address{}, AddressState(0), err
	}
	address := nextAddress(newState)
	return address, newState, nil
}

// returns the next state given an adressing state.
// The state values are incremented from 0 to 2^k-1
func nextState(state AddressState) (AddressState, error) {
	if uint64(state) > maxState {
		return AddressState(0), 
			fmt.Errorf("the state value is not valid, it must be less or equal to %x", maxState)
	}
	newState := state + 1
	return newState, nil
}

// uint64ToAddress returns an address with value v
// The value v fits into the address as the address size is 8
func uint64ToAddress(v uint64) Address {
	var b [AddressLength]byte
	binary.BigEndian.PutUint64(b[:], v)
	return Address(b)
}

// returns the next address given an adressing state
// The function assumes the state is valid (<2^k) so the test
// must be done before calling this function
func nextAddress(state AddressState) Address{
	index := uint64(state)
	address := uint64ToAddress(index)
	return address
}


