package handler

import (
	"encoding/binary"

	"github.com/onflow/atree"

	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

const ledgerAddressAllocatorKey = "AddressAllocator"

var (
	// leading zeros helps with storage
	FlowEVMPrecompileAddressPrefix = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	FlowEVMCOAAddressPrefix        = []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2}
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

	target := makeCOAAddress(uuid)

	// store new uuid
	newData := make([]byte, 8)
	binary.BigEndian.PutUint64(newData, uuid+1)
	err = aa.led.SetValue(aa.flexAddress[:], []byte(ledgerAddressAllocatorKey), newData)
	if err != nil {
		return types.Address{}, err
	}

	return target, nil
}

func makeCOAAddress(index uint64) types.Address {
	var addr types.Address
	copy(addr[:types.AddressLength-8], FlowEVMCOAAddressPrefix)
	binary.BigEndian.PutUint64(addr[types.AddressLength-8:], index)
	return addr
}

func (aa *AddressAllocator) AllocatePrecompileAddress(index uint64) types.Address {
	target := makePrecompileAddress(index)
	return target
}

func makePrecompileAddress(index uint64) types.Address {
	var addr types.Address
	copy(addr[:types.AddressLength-8], FlowEVMPrecompileAddressPrefix)
	binary.BigEndian.PutUint64(addr[types.AddressLength-8:], index)
	return addr
}
