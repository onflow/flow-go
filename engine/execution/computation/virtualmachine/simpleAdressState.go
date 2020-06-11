package virtualmachine

import (
	"encoding/binary"

	"github.com/dapperlabs/flow-go/model/flow"
)

// simpleAddressState implements an address generator that returns sequential addresses starting with 0x1
// (flow.Address{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x1} to be precise) and incrementing it by 1 for each next address.
// It is supposed to be used by the emulator for simpler reasoning about addresses.
type simpleAddressState struct {
	lastAddress uint64
}

func (s *simpleAddressState) Bytes() []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, s.lastAddress)
	return b
}

func (s *simpleAddressState) CurrentAddress() flow.Address {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, s.lastAddress)
	var addr flow.Address
	copy(addr[:], b)
	return addr
}

func (s *simpleAddressState) NextAddress() (flow.Address, error) {
	s.lastAddress += 1
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, s.lastAddress)
	var addr flow.Address
	copy(addr[:], b)
	return addr, nil
}

var _ AddressState = &simpleAddressState{}

func bytesToSimpleAddressState(b []byte) simpleAddressState {
	c := make([]byte, 8)
	copy(c, b)
	return simpleAddressState{binary.BigEndian.Uint64(c)}
}
