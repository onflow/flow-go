package handler

import (
	"encoding/binary"

	"github.com/onflow/atree"

	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

const ledgerAddressAllocatorKey = "AddressAllocator"

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

// AllocateAddress allocates an address
func (aa *AddressAllocator) AllocateAddress() (types.Address, error) {
	data, err := aa.led.GetValue(aa.flexAddress[:], []byte(ledgerAddressAllocatorKey))
	if err != nil {
		return types.Address{}, err
	}
	// default value for uuid is 1
	uuid := uint64(100)
	if len(data) > 0 {
		uuid = binary.BigEndian.Uint64(data)
	}

	target := types.Address{}
	// first 12 bytes would be zero
	// the next 8 bytes would be an increment of the UUID index
	binary.BigEndian.PutUint64(target[12:], uuid)

	// store new uuid
	newData := make([]byte, 8)
	binary.BigEndian.PutUint64(newData, uuid+1)
	err = aa.led.SetValue(aa.flexAddress[:], []byte(ledgerAddressAllocatorKey), newData)
	if err != nil {
		return types.Address{}, err
	}

	return target, nil
}
