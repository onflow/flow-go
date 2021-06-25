package systemcontracts

import (
	"testing"

	"github.com/onflow/flow-go/model/flow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSystemContract_Address tests that we can retrieve a canonical address
// for all accepted chains and contracts.
func TestSystemContract_Address(t *testing.T) {
	chains := []flow.ChainID{flow.Mainnet, flow.Testnet, flow.Canary, flow.Benchnet, flow.Localnet, flow.Emulator}
	contracts := []systemContract{Epoch, ClusterQC, DKG}

	for _, chain := range chains {
		for _, contract := range contracts {
			expected := systemContractsByChainID[chain][contract]
			actual, err := contract.Address(chain)
			require.NoError(t, err)
			require.Equal(t, expected, actual)
		}
	}
}

// TestSystemContract_InvalidChainID tests that we get an error if querying by an
// invalid chain ID.
func TestSystemContract_InvalidChainID(t *testing.T) {
	invalidChain := flow.ChainID("invalid-chain")
	contracts := []systemContract{Epoch, ClusterQC, DKG}

	for _, contract := range contracts {
		_, err := contract.Address(invalidChain)
		assert.Error(t, err)
	}
}
