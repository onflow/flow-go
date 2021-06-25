// Package systemcontracts stores canonical address locations for all system
// smart contracts.
//
// For transient networks, all system contracts can be deployed to the service
// account. For long-lived networks, system contracts are spread across several
// accounts for historical reasons.
package systemcontracts

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// systemContract represents a system smart contract owned by the service account.
type systemContract string

const (
	Epoch     systemContract = "FlowEpoch"
	ClusterQC systemContract = "FlowEpochClusterQC"
	DKG       systemContract = "FlowDKG"
)

// Address returns the canonical address of this system contract on the given chain.
func (sc systemContract) Address(chainID flow.ChainID) (flow.Address, error) {
	return systemContractAddressByChain(chainID, sc)
}

// String returns the string representation of the contract, which is also
// the name of the contract.
func (sc systemContract) String() string {
	return string(sc)
}

// systemContractsByChainID stores the default system smart contract
// addresses for each chain.
var systemContractsByChainID map[flow.ChainID]map[systemContract]flow.Address

func init() {
	systemContractsByChainID = make(map[flow.ChainID]map[systemContract]flow.Address)

	// Main Flow network
	mainnet := map[systemContract]flow.Address{
		Epoch:     flow.EmptyAddress,
		ClusterQC: flow.EmptyAddress,
		DKG:       flow.EmptyAddress,
	}
	systemContractsByChainID[flow.Mainnet] = mainnet

	// Long-lived test networks
	testnet := map[systemContract]flow.Address{
		Epoch:     flow.EmptyAddress,
		ClusterQC: flow.EmptyAddress,
		DKG:       flow.EmptyAddress,
	}
	systemContractsByChainID[flow.Testnet] = testnet
	systemContractsByChainID[flow.Canary] = testnet

	// Transient test networks
	transient := map[systemContract]flow.Address{
		Epoch:     flow.Emulator.Chain().ServiceAddress(),
		ClusterQC: flow.Emulator.Chain().ServiceAddress(),
		DKG:       flow.Emulator.Chain().ServiceAddress(),
	}
	systemContractsByChainID[flow.Emulator] = transient
	systemContractsByChainID[flow.Localnet] = transient
	systemContractsByChainID[flow.Benchnet] = transient
}

func systemContractAddressByChain(chainID flow.ChainID, contract systemContract) (flow.Address, error) {
	addresses, ok := systemContractsByChainID[chainID]
	if !ok {
		return flow.EmptyAddress, fmt.Errorf("unknown chain ID (%s)", chainID)
	}

	address, ok := addresses[contract]
	if !ok {
		return flow.EmptyAddress, fmt.Errorf("unknown contract (%s) for chain (%s)", contract, chainID)
	}

	return address, nil
}
