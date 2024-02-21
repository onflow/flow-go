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
	chains := flow.AllChainIDs()

	for _, chain := range chains {
		require.NotPanics(t, func() { SystemContractsForChain(chain) })
		checkSystemContracts(t, chain)
	}
}

// TestSystemContract_InvalidChainID tests that we get an error if querying by an
// invalid chain ID.
func TestSystemContract_InvalidChainID(t *testing.T) {
	invalidChain := flow.ChainID("invalid-chain")

	require.Panics(t, func() { SystemContractsForChain(invalidChain) })
}

// TestServiceEvents tests that we can retrieve service events for all accepted
// chains and contracts.
func TestServiceEvents(t *testing.T) {
	chains := flow.AllChainIDs()

	for _, chain := range chains {
		require.NotPanics(t, func() { ServiceEventsForChain(chain) })
		checkServiceEvents(t, chain)
	}
}

// TestServiceEventLookup_Consistency sanity checks consistency of the lookup
// method, in case an update to ServiceEvents forgets to update the lookup.
func TestServiceEventAll_Consistency(t *testing.T) {
	chains := flow.AllChainIDs()

	fields := reflect.TypeOf(ServiceEvents{}).NumField()
	for _, chain := range chains {
		events := ServiceEventsForChain(chain)

		// ensure all events are present
		all := events.All()
		assert.Equal(t, fields, len(all))
	}
}

// TestServiceEvents_InvalidChainID tests that we get an error if querying by an
// invalid chain ID.
func TestServiceEvents_InvalidChainID(t *testing.T) {
	invalidChain := flow.ChainID("invalid-chain")

	require.Panics(t, func() { ServiceEventsForChain(invalidChain) })
}

func checkSystemContracts(t *testing.T, chainID flow.ChainID) {
	contracts := SystemContractsForChain(chainID)

	address := func(name string) flow.Address {
		f, ok := contractAddressFunc[name]
		require.True(t, ok, "missing contract %s for chain %s", name, chainID.String())
		return f(chainID)
	}

	// entries may not be empty
	assert.NotEqual(t, flow.EmptyAddress, address(ContractNameEpoch))
	assert.NotEqual(t, flow.EmptyAddress, address(ContractNameClusterQC))
	assert.NotEqual(t, flow.EmptyAddress, address(ContractNameDKG))
	assert.NotEqual(t, flow.EmptyAddress, address(ContractNameNodeVersionBeacon))

	// entries must match internal mapping
	assert.Equal(t, address(ContractNameEpoch), contracts.Epoch.Address)
	assert.Equal(t, address(ContractNameClusterQC), contracts.ClusterQC.Address)
	assert.Equal(t, address(ContractNameDKG), contracts.DKG.Address)
	assert.Equal(t, address(ContractNameNodeVersionBeacon), contracts.NodeVersionBeacon.Address)
}

func checkServiceEvents(t *testing.T, chainID flow.ChainID) {
	events := ServiceEventsForChain(chainID)

	address := func(name string) flow.Address {
		f, ok := contractAddressFunc[name]
		require.True(t, ok, "missing contract %s for chain %s", name, chainID.String())
		return f(chainID)
	}

	epochContractAddr := address(ContractNameEpoch)
	versionContractAddr := address(ContractNameNodeVersionBeacon)
	serviceAccountAddr := address(ContractNameServiceAccount)
	// entries may not be empty
	assert.NotEqual(t, flow.EmptyAddress, epochContractAddr)
	assert.NotEqual(t, flow.EmptyAddress, versionContractAddr)
	assert.NotEqual(t, flow.EmptyAddress, serviceAccountAddr)

	// entries must match internal mapping
	assert.Equal(t, epochContractAddr, events.EpochSetup.Address)
	assert.Equal(t, epochContractAddr, events.EpochCommit.Address)
	assert.Equal(t, versionContractAddr, events.VersionBeacon.Address)
	assert.Equal(t, serviceAccountAddr, events.ProtocolStateVersionUpgrade.Address)
}
