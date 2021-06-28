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
		checkSystemContracts(t, chain)
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
		checkServiceEvents(t, chain)
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

func checkSystemContracts(t *testing.T, chainID flow.ChainID) {
	contracts, err := SystemContractsForChain(chainID)
	require.NoError(t, err)

	addresses, ok := contractAddressesByChainID[chainID]
	require.True(t, ok, "missing chain %s", chainID.String())

	// entries may not be empty
	assert.NotEqual(t, flow.EmptyAddress, addresses[ContractNameEpoch])
	assert.NotEqual(t, flow.EmptyAddress, addresses[ContractNameClusterQC])
	assert.NotEqual(t, flow.EmptyAddress, addresses[ContractNameDKG])

	// entries must match internal mapping
	assert.Equal(t, addresses[ContractNameEpoch], contracts.Epoch.Address)
	assert.Equal(t, addresses[ContractNameClusterQC], contracts.ClusterQC.Address)
	assert.Equal(t, addresses[ContractNameDKG], contracts.DKG.Address)
}

func checkServiceEvents(t *testing.T, chainID flow.ChainID) {
	events, err := ServiceEventsForChain(chainID)
	require.NoError(t, err)

	addresses, ok := contractAddressesByChainID[chainID]
	require.True(t, ok, "missing chain %w", chainID.String())

	epochContractAddr := addresses[ContractNameEpoch]
	// entries may not be empty
	assert.NotEqual(t, flow.EmptyAddress, epochContractAddr)

	// entries should match expected format
	assert.Equal(t, serviceEventQualifiedType(eventEpochSetupFormat, epochContractAddr), events.EpochSetup.QualifiedType)
	assert.Equal(t, serviceEventQualifiedType(eventEpochCommitFormat, epochContractAddr), events.EpochCommit.QualifiedType)
}
