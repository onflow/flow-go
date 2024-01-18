package types

import (
	"math/big"

	gethCommon "github.com/onflow/go-ethereum/common"
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

type GasLimit uint64

type Code []byte

type Data []byte

// AsBigInt process the data and return it as a big integer
func (d Data) AsBigInt() *big.Int {
	return new(big.Int).SetBytes(d)
}
