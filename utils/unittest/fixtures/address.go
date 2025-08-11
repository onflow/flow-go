package fixtures

import (
	"testing"

	sdk "github.com/onflow/flow-go-sdk"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
)

// AddressGenerator generates addresses with consistent randomness.
type AddressGenerator struct {
	randomGen *RandomGenerator
}

// addressConfig holds the configuration for address generation.
type addressConfig struct {
	chainID flow.ChainID
	index   uint64
}

// ForChain returns an option to generate an address for a specific chain.
func (g *AddressGenerator) WithChainID(chainID flow.ChainID) func(*addressConfig) {
	return func(config *addressConfig) {
		config.chainID = chainID
	}
}

func (g *AddressGenerator) ServiceAddress() func(*addressConfig) {
	return func(config *addressConfig) {
		config.index = 1
	}
}

// Fixture generates a random address with optional configuration.
// Defaults to Testnet if no chain is specified.
func (g *AddressGenerator) Fixture(t testing.TB, opts ...func(*addressConfig)) flow.Address {
	config := &addressConfig{
		chainID: flow.Testnet,
		// we use a 32-bit index - since the linear address generator uses 45 bits,
		// this won't error
		index: uint64(g.randomGen.Uint32()),
	}

	// Apply options
	for _, opt := range opts {
		opt(config)
	}

	addr, err := config.chainID.Chain().AddressAtIndex(config.index)
	require.NoError(t, err)

	return addr
}

// ToSDKAddress converts a flow.Address to a sdk.Address.
func ToSDKAddress(addr flow.Address) sdk.Address {
	var sdkAddr sdk.Address
	copy(sdkAddr[:], addr[:])
	return sdkAddr
}

// CorruptAddress corrupts the first byte of the address.
func CorruptAddress(t testing.TB, addr flow.Address) flow.Address {
	addr[0] ^= 1
	require.False(t, flow.Testnet.Chain().IsValid(addr), "invalid address fixture generated valid address")
	return addr
}
