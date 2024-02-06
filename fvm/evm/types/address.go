package types

import (
	"bytes"

	gethCommon "github.com/ethereum/go-ethereum/common"
)

// FlowEVMSpecialAddressPrefixLen captures the number of prefix bytes with constant values for special accounts (extended precompiles and COAs).
//
// The prefix length should insure a high-enough level of security against finding a preimage using the hash
// function used for EVM addresses generation (Keccak256). This is required to avoid finding an EVM address
// that is also a valid FlowEVM address.
// The target (minimal) security in this case is the security level provided by EVM addresses.
// Since EVM addresses are 160-bits long, they offer only 80 bits of security (collision resistance
// offers the lowest level).
// A preimage resistance of 80 bits requires the prefix to be at least 80-bits long (i.e 10 bytes).
// When used as a prefix in EVM addresses (20-bytes long), a prefix length of 12 bytes
// leaves a variable part of 8 bytes (64 bits).
const FlowEVMSpecialAddressPrefixLen = 12

var (
	// Using leading zeros for prefix helps with the storage compactness.
	//
	// Prefix for the built-in EVM precompiles
	FlowEVMNativePrecompileAddressPrefix = [FlowEVMSpecialAddressPrefixLen]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	// Prefix for the extended precompiles
	FlowEVMExtendedPrecompileAddressPrefix = [FlowEVMSpecialAddressPrefixLen]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	// Prefix for the COA addresses
	FlowEVMCOAAddressPrefix = [FlowEVMSpecialAddressPrefixLen]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2}
)

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

// IsACOAAddress returns true if the address is a COA address
//
// This test insures `addr` has been generated as a COA address with high probability.
// Brute forcing an EVM address `addr` to pass the `IsACOAAddress` test is as hard as the bit-length
// of `FlowEVMCOAAddressPrefix` (here 96 bits).
// Although this is lower than the protocol-wide security level in Flow (128 bits), it remains
// higher than the EVM addresses security (80 bits when considering collision attacks)
func IsACOAAddress(addr Address) bool {
	return bytes.HasPrefix(addr[:], FlowEVMCOAAddressPrefix[:])
}

// IsAnExtendedPrecompileAddress returns true if the address is a extended precompile address
func IsAnExtendedPrecompileAddress(addr Address) bool {
	return bytes.HasPrefix(addr[:], FlowEVMExtendedPrecompileAddressPrefix[:])
}
