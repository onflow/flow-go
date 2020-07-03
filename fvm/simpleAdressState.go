package fvm

import (
	"encoding/binary"

	"github.com/dapperlabs/flow-go/model/flow"
)

// SimpleAddressState implements an address generator that returns sequential addresses starting with 0x1
// (flow.Address{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x1} to be precise) and incrementing it by 1 for each next address.
// It is supposed to be used by the emulator for simpler reasoning about addresses.
type SimpleAddressState struct {
	lastAddress uint64
}

func (s *SimpleAddressState) Bytes() []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, s.lastAddress)
	return b
}

func (s *SimpleAddressState) CurrentAddress() flow.Address {
	return SimpleAddressAtIndex(s.lastAddress)
}

func (s *SimpleAddressState) NextAddress() (flow.Address, error) {
	s.lastAddress += 1
	return SimpleAddressAtIndex(s.lastAddress), nil
}

var _ AddressState = &SimpleAddressState{}

func BytesToSimpleAddressState(b []byte) SimpleAddressState {
	c := make([]byte, 8)
	copy(c, b)
	return SimpleAddressState{binary.BigEndian.Uint64(c)}
}

// SimpleAddressAtIndex returns the nth generated account address.
func SimpleAddressAtIndex(index uint64) flow.Address {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, index)
	var addr flow.Address
	copy(addr[:], b)
	return addr
}

// SimpleServiceAddress returns the root (first) generated account address.
func SimpleServiceAddress() flow.Address {
	return SimpleAddressAtIndex(1)
}

func SimpleFungibleTokenAddress() flow.Address {
	return SimpleAddressAtIndex(2)
}

func SimpleFlowTokenAddress() flow.Address {
	return SimpleAddressAtIndex(3)
}
