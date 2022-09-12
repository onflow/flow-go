package systemcontracts

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
)

// TestSystemContract_Address tests that we can retrieve a canonical address
// for all accepted chains and contracts.
func TestSystemContracts(t *testing.T) {
	chains := []flow.ChainID{
		flow.Mainnet, flow.Testnet, flow.Stagingnet,
		flow.Benchnet, flow.Localnet, flow.Emulator,
		flow.BftTestnet, flow.MonotonicEmulator}

	for _, chain := range chains {
		_ = SystemContractsForChain(chain)
		checkSystemContracts(t, chain)
	}
}

// TestSystemContract_InvalidChainID tests that we panic when trying to get system contracts for an unknown chain
func TestSystemContract_InvalidChainID(t *testing.T) {
	invalidChain := flow.ChainID("invalid-chain")

	require.Panics(t, func() {
		_ = SystemContractsForChain(invalidChain)
	})
}

// TestServiceEvents tests that we can retrieve service events for all accepted
// chains and contracts.
func TestServiceEvents(t *testing.T) {
	chains := []flow.ChainID{
		flow.Mainnet, flow.Testnet, flow.Stagingnet,
		flow.Benchnet, flow.Localnet, flow.Emulator,
		flow.BftTestnet, flow.MonotonicEmulator}

	for _, chain := range chains {
		_ = ServiceEventsForChain(chain)
		checkServiceEvents(t, chain)
	}
}

// TestServiceEventLookup_Consistency sanity checks consistency of the lookup
// method, in case an update to ServiceEvents forgets to update the lookup.
func TestServiceEventAll_Consistency(t *testing.T) {
	chains := []flow.ChainID{
		flow.Mainnet, flow.Testnet, flow.Stagingnet,
		flow.Benchnet, flow.Localnet, flow.Emulator,
		flow.BftTestnet, flow.MonotonicEmulator}

	fields := reflect.TypeOf(ServiceEvents{}).NumField()
	for _, chain := range chains {
		events := ServiceEventsForChain(chain)

		// ensure all events are returns
		all := events.All()
		assert.Equal(t, fields, len(all))
	}
}

// TestServiceEvents_InvalidChainID tests that we panic when trying to get system contracts for an unknown chain
func TestServiceEvents_InvalidChainID(t *testing.T) {
	invalidChain := flow.ChainID("invalid-chain")

	require.Panics(t, func() {
		_ = ServiceEventsForChain(invalidChain)
	})
}

func checkSystemContracts(t *testing.T, chainID flow.ChainID) {
	contracts := SystemContractsForChain(chainID)

	addresses, ok := contractAddressesByChainID[chainID]
	require.True(t, ok, "missing chain %s", chainID.String())

	// entries may not be empty
	assert.NotEqual(t, flow.EmptyAddress, addresses[ContractNameEpoch])
	assert.NotEqual(t, flow.EmptyAddress, addresses[ContractNameClusterQC])
	assert.NotEqual(t, flow.EmptyAddress, addresses[ContractNameDKG])
	assert.NotEqual(t, flow.EmptyAddress, addresses[ContractNameFlowFees])
	assert.NotEqual(t, flow.EmptyAddress, addresses[ContractNameFungibleToken])
	assert.NotEqual(t, flow.EmptyAddress, addresses[ContractNameFlowToken])

	// entries must match internal mapping
	assert.Equal(t, addresses[ContractNameEpoch], contracts.Epoch.Address)
	assert.Equal(t, addresses[ContractNameClusterQC], contracts.ClusterQC.Address)
	assert.Equal(t, addresses[ContractNameDKG], contracts.DKG.Address)
	assert.Equal(t, addresses[ContractNameFlowFees], contracts.Fees.Address)
	assert.Equal(t, addresses[ContractNameFungibleToken], contracts.FungibleToken.Address)
	assert.Equal(t, addresses[ContractNameFlowToken], contracts.FlowToken.Address)
}

func checkServiceEvents(t *testing.T, chainID flow.ChainID) {
	events := ServiceEventsForChain(chainID)

	addresses, ok := contractAddressesByChainID[chainID]
	require.True(t, ok, "missing chain %w", chainID.String())

	epochContractAddr := addresses[ContractNameEpoch]
	// entries may not be empty
	assert.NotEqual(t, flow.EmptyAddress, epochContractAddr)

	// entries must match internal mapping
	assert.Equal(t, epochContractAddr, events.EpochSetup.Address)
	assert.Equal(t, epochContractAddr, events.EpochCommit.Address)
}
