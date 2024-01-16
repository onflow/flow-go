package handler

import (
	"encoding/binary"

	"github.com/onflow/atree"

	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

const (
	ledgerAddressAllocatorKey = "AddressAllocator"
	uint64ByteSize            = 8
)

var (
	// prefixes:
	// the first 12 bytes of addresses allocation
	// leading zeros helps with storage and all zero is reserved for the EVM precompiles
	FlowEVMPrecompileAddressPrefix = [...]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	FlowEVMCOAAddressPrefix        = [...]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2}
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

// AllocateCOAAddress allocates an address for COA
func (aa *AddressAllocator) AllocateCOAAddress() (types.Address, error) {
	data, err := aa.led.GetValue(aa.flexAddress[:], []byte(ledgerAddressAllocatorKey))
	if err != nil {
		return types.Address{}, err
	}
	// default value for uuid is 1
	uuid := uint64(1)
	if len(data) > 0 {
		uuid = binary.BigEndian.Uint64(data)
	}

	target := MakeCOAAddress(uuid)

	// store new uuid
	newData := make([]byte, 8)
	binary.BigEndian.PutUint64(newData, uuid+1)
	err = aa.led.SetValue(aa.flexAddress[:], []byte(ledgerAddressAllocatorKey), newData)
	if err != nil {
		return types.Address{}, err
	}

	return target, nil
}

func MakeCOAAddress(index uint64) types.Address {
	var addr types.Address
	prefixIndex := types.AddressLength - uint64ByteSize
	copy(addr[:prefixIndex], FlowEVMCOAAddressPrefix[:])
	binary.BigEndian.PutUint64(addr[prefixIndex:], index)
	return addr
}

func (aa *AddressAllocator) AllocatePrecompileAddress(index uint64) types.Address {
	target := MakePrecompileAddress(index)
	return target
}

func MakePrecompileAddress(index uint64) types.Address {
	var addr types.Address
	prefixIndex := types.AddressLength - uint64ByteSize
	copy(addr[:prefixIndex], FlowEVMPrecompileAddressPrefix[:])
	binary.BigEndian.PutUint64(addr[prefixIndex:], index)
	return addr
}
