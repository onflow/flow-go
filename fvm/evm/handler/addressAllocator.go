package handler

import (
	"encoding/binary"

	"github.com/onflow/atree"

	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

const (
	// `addressIndexMultiplierConstant` is used for mapping address indices
	// into deterministic random-looking address postfixes.
	// The constant must be an ODD number.
	// It is a "nothing-up-my-sleeves" constant, chosen to be big enough so that
	// the index and its corresponding address look less "related".
	// Note that the least significant byte was set to "77" instead of "88" to force
	// the odd parity.
	// Look at `mapAddressIndex` for more details.
	addressIndexMultiplierConstant = uint64(0xFFEEDDCCBBAA9977)
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
	return makePrefixedAddress(mapAddressIndex(index), types.FlowEVMCOAAddressPrefix)
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

// `mapAddressIndex` maps an index of 64 bits to a deterministic random-looking 64 bits.
//
// The mapping function must be an injective mapping (in this case bijective)
// where every two indices always map to two different results. Multiple injective
// mappings are possible.
//
// The current implementation uses a simple modular multiplication by a constant modulo 2^64.
// The multiplier constant can be any odd number. Since odd numbers are co-prime with 2^64, they
// have a multiplicative inverse modulo 2^64.
// This makes multiplying by an odd number an injective function (and therefore bijective).
//
// Multiplying modulo 2^64 is implicitly implemented as a uint64 multiplication with a uin64 result.
func mapAddressIndex(index uint64) uint64 {
	return uint64(index * addressIndexMultiplierConstant)
}
