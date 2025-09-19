package fixtures

import (
	sdk "github.com/onflow/flow-go-sdk"

	"github.com/onflow/flow-go/model/flow"
)

// maxIndex is the maximum index for the linear code address generator.
const maxIndex = 1<<45 - 1

// Address is the default options factory for [flow.Address] generation.
var Address addressFactory

type addressFactory struct{}

type AddressOption func(*AddressGenerator, *addressConfig)

// addressConfig holds the configuration for address generation.
type addressConfig struct {
	chainID flow.ChainID
	index   uint64
}

// WithChainID is an option that generates an [flow.Address] for the specified chain.
func (f addressFactory) WithChainID(chainID flow.ChainID) AddressOption {
	return func(g *AddressGenerator, config *addressConfig) {
		config.chainID = chainID
	}
}

// ServiceAddress is an option that generates the service account [flow.Address] for the given chain.
func (f addressFactory) ServiceAddress() AddressOption {
	return func(g *AddressGenerator, config *addressConfig) {
		config.index = 1
	}
}

// WithIndex is an option that sets the index for the address.
func (f addressFactory) WithIndex(index uint64) AddressOption {
	return func(g *AddressGenerator, config *addressConfig) {
		config.index = index
	}
}

// AddressGenerator generates [flow.Address] with consistent randomness.
type AddressGenerator struct {
	addressFactory

	random *RandomGenerator

	chainID flow.ChainID
}

func NewAddressGenerator(
	random *RandomGenerator,
	chainID flow.ChainID,
) *AddressGenerator {
	return &AddressGenerator{
		random:  random,
		chainID: chainID,
	}
}

// Fixture generates a random [flow.Address] with the provided options.
// Defaults to the chain ID specified in the generator suite.
func (g *AddressGenerator) Fixture(opts ...AddressOption) flow.Address {
	config := &addressConfig{
		chainID: g.chainID,
		index:   g.random.Uint64InRange(1, maxIndex),
	}

	for _, opt := range opts {
		opt(g, config)
	}

	Assertf(config.index <= maxIndex, "index must be less than %d, got %d", maxIndex, config.index)

	addr, err := config.chainID.Chain().AddressAtIndex(config.index)
	NoError(err)

	return addr
}

// List returns a list of [flow.Address] with the provided options.
func (g *AddressGenerator) List(n int, opts ...AddressOption) []flow.Address {
	addresses := make([]flow.Address, n)
	for i := range uint64(n) {
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
	// this should only fail if the provided address was already not valid for the chain. explicitly
	// check so the returned address is guaranteed to be invalid.
	Assert(!chainID.Chain().IsValid(addr), "corrupted address is valid!")
	return addr
}
