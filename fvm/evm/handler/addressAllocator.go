package handler

import (
	"encoding/binary"

	"github.com/onflow/atree"

	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

const (
	ledgerAddressAllocatorKey = "AddressAllocator"
	// addressIndexShuffleSeed is used for shuffling address index
	// shuffling index is used to make address postfixes look random
	addressIndexShuffleSeed = uint64(0xFFEEDDCCBBAA9987)
)

type AddressAllocator struct {
	led         atree.Ledger
	flexAddress flow.Address
}

var _ types.AddressAllocator = &AddressAllocator{}

// NewAddressAllocator constructs a new statefull address allocator
func NewAddressAllocator(led atree.Ledger, flexAddress flow.Address) (*AddressAllocator, error) {
	return &AddressAllocator{
		led:         led,
		flexAddress: flexAddress,
	}, nil
}

func (aa *AddressAllocator) COAFactoryAddress() types.Address {
	return MakeCOAAddress(0)
}

// AllocateCOAAddress allocates an address for COA
func (aa *AddressAllocator) AllocateCOAAddress(uuid uint64) types.Address {
	return MakeCOAAddress(uuid)
}

func MakeCOAAddress(index uint64) types.Address {
	return makePrefixedAddress(shuffleAddressIndex(index), types.FlowEVMCOAAddressPrefix)
}

func (aa *AddressAllocator) AllocatePrecompileAddress(index uint64) types.Address {
	target := MakePrecompileAddress(index)
	return target
}

func MakePrecompileAddress(index uint64) types.Address {
	return makePrefixedAddress(index, types.FlowEVMExtendedPrecompileAddressPrefix)
}

func makePrefixedAddress(
	index uint64,
	prefix [types.FlowEVMSpecialAddressPrefixLen]byte,
) types.Address {
	var addr types.Address
	copy(addr[:], prefix[:])
	// only works if `len(addr) - len(prefix)` is exactly 8 bytes
	binary.BigEndian.PutUint64(addr[len(prefix):], index)
	return addr
}

func shuffleAddressIndex(preShuffleIndex uint64) uint64 {
	return uint64(preShuffleIndex * addressIndexShuffleSeed)
}
