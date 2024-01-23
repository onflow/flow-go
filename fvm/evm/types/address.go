package types

import (
	"bytes"

	gethCommon "github.com/ethereum/go-ethereum/common"
)

const (
	// number of prefix bytes with specific values for special accounts (extended precompiles and COAs)
	// using leading zeros for prefix helps with the storage compactness
	FlowEVMSpecialAddressPrefixLen = 12
)

var (
	// Prefix for the built-in EVM precompiles
	FlowEVMNativePrecompileAddressPrefix = [FlowEVMSpecialAddressPrefixLen]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	// Prefix for the extended precompiles
	FlowEVMExtendedPrecompileAddressPrefix = [FlowEVMSpecialAddressPrefixLen]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	// Prefix for the COA addresses
	FlowEVMCOAAddressPrefix = [FlowEVMSpecialAddressPrefixLen]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2}
)

// IsACOAAddress returns true if the address is a COA address
func IsACOAAddress(addr Address) bool {
	return bytes.HasPrefix(addr[:], FlowEVMCOAAddressPrefix[:])
}

// IsACOAAddress returns true if the address is a COA address
func IsAnExtendedPrecompileAddress(addr Address) bool {
	return bytes.HasPrefix(addr[:], FlowEVMExtendedPrecompileAddressPrefix[:])
}

// Address is an EVM-compatible address
type Address gethCommon.Address

// AddressLength holds the number of bytes used for each EVM address
const AddressLength = gethCommon.AddressLength

// NewAddress constructs a new Address
func NewAddress(addr gethCommon.Address) Address {
	return Address(addr)
}

// EmptyAddress is an empty evm address
var EmptyAddress = Address(gethCommon.Address{})

// Bytes returns a byte slice for the address
func (fa Address) Bytes() []byte {
	return fa[:]
}

// ToCommon returns the geth address
func (fa Address) ToCommon() gethCommon.Address {
	return gethCommon.Address(fa)
}

// NewAddressFromBytes constructs a new address from bytes
func NewAddressFromBytes(inp []byte) Address {
	return Address(gethCommon.BytesToAddress(inp))
}

// NewAddressFromString constructs a new address from an string
func NewAddressFromString(str string) Address {
	return NewAddressFromBytes([]byte(str))
}
