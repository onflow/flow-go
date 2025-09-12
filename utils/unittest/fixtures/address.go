package fixtures

import (
	sdk "github.com/onflow/flow-go-sdk"

	"github.com/onflow/flow-go/model/flow"
)

// AddressGenerator generates addresses with consistent randomness.
type AddressGenerator struct {
	randomGen *RandomGenerator

	chainID flow.ChainID
}

func NewAddressGenerator(
	randomGen *RandomGenerator,
	chainID flow.ChainID,
) *AddressGenerator {
	return &AddressGenerator{
		randomGen: randomGen,
		chainID:   chainID,
	}
}

// addressConfig holds the configuration for address generation.
type addressConfig struct {
	chainID flow.ChainID
	index   uint64
}

// WithChainID is an option that generates an [flow.Address] for the specified chain.
func (g *AddressGenerator) WithChainID(chainID flow.ChainID) func(*addressConfig) {
	return func(config *addressConfig) {
		config.chainID = chainID
	}
}

// ServiceAddress is an option that generates the service account [flow.Address] for the given chain.
func (g *AddressGenerator) ServiceAddress() func(*addressConfig) {
	return func(config *addressConfig) {
		config.index = 1
	}
}

// WithIndex is an option that sets the index for the address.
func (g *AddressGenerator) WithIndex(index uint64) func(*addressConfig) {
	return func(config *addressConfig) {
		config.index = index
	}
}

// Fixture generates a random [flow.Address] with the provided options.
// Defaults to the chain ID specified in the generator suite.
func (g *AddressGenerator) Fixture(opts ...func(*addressConfig)) flow.Address {
	config := &addressConfig{
		chainID: g.chainID,
		// we use a 32-bit index - since the linear address generator uses 45 bits,
		// this won't error
		index: uint64(g.randomGen.Uint32()),
	}

	for _, opt := range opts {
		opt(config)
	}

	addr, err := config.chainID.Chain().AddressAtIndex(config.index)
	NoError(err)

	return addr
}

// List returns a list of [flow.Address] with the provided options.
func (g *AddressGenerator) List(n int, opts ...func(*addressConfig)) []flow.Address {
	addresses := make([]flow.Address, n)
	for i := range uint64(n) {
		// set the index explicitly to guarantee we do not have duplicates
		opts = append(opts, g.WithIndex(g.randomGen.Uint64InRange(i, i+1000)))
		addresses[i] = g.Fixture(opts...)
	}
	return addresses
}

// ToSDKAddress converts a [flow.Address] to a [sdk.Address].
func ToSDKAddress(addr flow.Address) sdk.Address {
	var sdkAddr sdk.Address
	copy(sdkAddr[:], addr[:])
	return sdkAddr
}

// CorruptAddress corrupts the first byte of the address and checks that the address is invalid.
func CorruptAddress(addr flow.Address, chainID flow.ChainID) flow.Address {
	addr[0] ^= 1
	Assert(!chainID.Chain().IsValid(addr), "invalid address fixture generated valid address")
	return addr
}
