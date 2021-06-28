package systemcontracts

import (
	"testing"

	"github.com/onflow/flow-go/model/flow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSystemContract_Address tests that we can retrieve a canonical address
// for all accepted chains and contracts.
func TestSystemContracts(t *testing.T) {
	chains := []flow.ChainID{flow.Mainnet, flow.Testnet, flow.Canary, flow.Benchnet, flow.Localnet, flow.Emulator}

	for _, chain := range chains {
		_, err := SystemContractsForChain(chain)
		require.NoError(t, err)
	}
}

// TestSystemContract_InvalidChainID tests that we get an error if querying by an
// invalid chain ID.
func TestSystemContract_InvalidChainID(t *testing.T) {
	invalidChain := flow.ChainID("invalid-chain")

	_, err := SystemContractsForChain(invalidChain)
	assert.Error(t, err)
}

// TestServiceEvents tests that we can retrieve service events for all accepted
// chains and contracts.
func TestServiceEvents(t *testing.T) {
	chains := []flow.ChainID{flow.Mainnet, flow.Testnet, flow.Canary, flow.Benchnet, flow.Localnet, flow.Emulator}

	for _, chain := range chains {
		_, err := ServiceEventsForChain(chain)
		require.NoError(t, err)
	}
}

// TestServiceEvents_InvalidChainID tests that we get an error if querying by an
// invalid chain ID.
func TestServiceEvents_InvalidChainID(t *testing.T) {
	invalidChain := flow.ChainID("invalid-chain")

	_, err := ServiceEventsForChain(invalidChain)
	assert.Error(t, err)
}
