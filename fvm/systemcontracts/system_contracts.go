// Package systemcontracts stores canonical address locations for all system
// smart contracts and service events.
//
// System contracts are special smart contracts controlled by the service account,
// a Flow account with special privileges to administer the network.
//
// Service events are special events defined within system contracts which
// are included within execution receipts and processed by the consensus committee
// to enable message-passing to the protocol state.
//
// For transient networks, all system contracts can be deployed to the service
// account. For long-lived networks, system contracts are spread across several
// accounts for historical reasons.
package systemcontracts

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

const (

	// Unqualified names of system smart contracts (not including address prefix)

	ContractNameEpoch         = "FlowEpoch"
	ContractNameClusterQC     = "FlowClusterQC"
	ContractNameDKG           = "FlowDKG"
	ContractServiceAccount    = "FlowServiceAccount"
	ContractNameFlowFees      = "FlowFees"
	ContractStorageFees       = "FlowStorageFees"
	ContractDeploymentAudits  = "FlowContractAudits"
	ContractNameFungibleToken = "FungibleToken"
	ContractNameFlowToken     = "FlowToken"

	// Unqualified names of service events (not including address prefix or contract name)

	EventNameEpochSetup  = "EpochSetup"
	EventNameEpochCommit = "EpochCommit"

	//  Unqualified names of service event contract functions (not including address prefix or contract name)

	ContractServiceAccountFunction_setupNewAccount           = "setupNewAccount"
	ContractServiceAccountFunction_defaultTokenBalance       = "defaultTokenBalance"
	ContractServiceAccountFunction_deductTransactionFee      = "deductTransactionFee"
	ContractStorageFeesFunction_calculateAccountCapacity     = "calculateAccountCapacity"
	ContractStorageFeesFunction_calculateAccountsCapacity    = "calculateAccountsCapacity"
	ContractStorageFeesFunction_defaultTokenAvailableBalance = "defaultTokenAvailableBalance"
	ContractDeploymentAuditsFunction_useVoucherForDeploy     = "useVoucherForDeploy"
)

const (
	fungibleTokenAccountIndex = 2
	flowTokenAccountIndex     = 3
	flowFeesAccountIndex      = 4
)

// SystemContract represents a system contract on a particular chain.
type SystemContract struct {
	Address flow.Address
	Name    string
}

// ServiceEvent represents a service event on a particular chain.
type ServiceEvent struct {
	Address      flow.Address
	ContractName string
	Name         string
}

// QualifiedIdentifier returns the Cadence qualified identifier of the service
// event, which includes the contract name and the event type name.
func (se ServiceEvent) QualifiedIdentifier() string {
	return fmt.Sprintf("%s.%s", se.ContractName, se.Name)
}

// EventType returns the full event type identifier, including the address, the
// contract name, and the event type name.
func (se ServiceEvent) EventType() flow.EventType {
	return flow.EventType(fmt.Sprintf("A.%s.%s.%s", se.Address, se.ContractName, se.Name))
}

// SystemContracts is a container for all system contracts on a particular chain.
type SystemContracts struct {
	Epoch         SystemContract
	ClusterQC     SystemContract
	DKG           SystemContract
	Fees          SystemContract
	FungibleToken SystemContract
	FlowToken     SystemContract
}

// ServiceEvents is a container for all service events on a particular chain.
type ServiceEvents struct {
	EpochSetup  ServiceEvent
	EpochCommit ServiceEvent
}

// All returns all service events as a slice.
func (se ServiceEvents) All() []ServiceEvent {
	return []ServiceEvent{
		se.EpochSetup,
		se.EpochCommit,
	}
}

// SystemContractsForChain returns the system contract configuration for the given chain.
func SystemContractsForChain(chainID flow.ChainID) *SystemContracts {
	addresses, ok := contractAddressesByChainID[chainID]
	if !ok {
		panic("contractAddressesByChainID are not set up for chain " + chainID.String())
	}

	contracts := &SystemContracts{
		Epoch: SystemContract{
			Address: addresses[ContractNameEpoch],
			Name:    ContractNameEpoch,
		},
		ClusterQC: SystemContract{
			Address: addresses[ContractNameClusterQC],
			Name:    ContractNameClusterQC,
		},
		DKG: SystemContract{
			Address: addresses[ContractNameDKG],
			Name:    ContractNameDKG,
		},
		Fees: SystemContract{
			Address: addresses[ContractNameFlowFees],
			Name:    ContractNameFlowFees,
		},
		FungibleToken: SystemContract{
			Address: addresses[ContractNameFungibleToken],
			Name:    ContractNameFungibleToken,
		},
		FlowToken: SystemContract{
			Address: addresses[ContractNameFlowToken],
			Name:    ContractNameFlowToken,
		},
	}

	return contracts
}

// ServiceEventsForChain returns the service event confirmation for the given chain.
func ServiceEventsForChain(chainID flow.ChainID) *ServiceEvents {
	addresses, ok := contractAddressesByChainID[chainID]
	if !ok {
		panic("contractAddressesByChainID are not set up for chain " + chainID.String())
	}

	events := &ServiceEvents{
		EpochSetup: ServiceEvent{
			Address:      addresses[ContractNameEpoch],
			ContractName: ContractNameEpoch,
			Name:         EventNameEpochSetup,
		},
		EpochCommit: ServiceEvent{
			Address:      addresses[ContractNameEpoch],
			ContractName: ContractNameEpoch,
			Name:         EventNameEpochCommit,
		},
	}

	return events
}

// contractAddressesByChainID stores the default system smart contract
// addresses for each chain.
var contractAddressesByChainID map[flow.ChainID]map[string]flow.Address

// Well-known addresses for system contracts on long-running networks.
// For now, all system contracts tracked by this package are deployed to the same
// address (per chain) as the staking contract.
//
// Ref: https://docs.onflow.org/core-contracts/staking-contract-reference/
var (
	// stakingContractAddressMainnet is the address of the FlowIDTableStaking contract on Mainnet
	stakingContractAddressMainnet = flow.HexToAddress("8624b52f9ddcd04a")
	// stakingContractAddressTestnet is the address of the FlowIDTableStaking contract on Testnet
	stakingContractAddressTestnet = flow.HexToAddress("9eca2b38b18b5dfe")
)

func init() {
	contractAddressesByChainID = make(map[flow.ChainID]map[string]flow.Address)

	// Main Flow network
	// All system contracts are deployed to the account of the staking contract
	mainnet := map[string]flow.Address{
		ContractNameEpoch:         stakingContractAddressMainnet,
		ContractNameClusterQC:     stakingContractAddressMainnet,
		ContractNameDKG:           stakingContractAddressMainnet,
		ContractNameFlowFees:      mustAddressAtIndex(flow.Mainnet.Chain(), flowFeesAccountIndex),
		ContractNameFungibleToken: mustAddressAtIndex(flow.Mainnet.Chain(), fungibleTokenAccountIndex),
		ContractNameFlowToken:     mustAddressAtIndex(flow.Mainnet.Chain(), flowTokenAccountIndex),
	}
	contractAddressesByChainID[flow.Mainnet] = mainnet

	// Long-lived test networks
	// All system contracts are deployed to the account of the staking contract
	testnet := map[string]flow.Address{
		ContractNameEpoch:         stakingContractAddressTestnet,
		ContractNameClusterQC:     stakingContractAddressTestnet,
		ContractNameDKG:           stakingContractAddressTestnet,
		ContractNameFlowFees:      mustAddressAtIndex(flow.Testnet.Chain(), flowFeesAccountIndex),
		ContractNameFungibleToken: mustAddressAtIndex(flow.Testnet.Chain(), fungibleTokenAccountIndex),
		ContractNameFlowToken:     mustAddressAtIndex(flow.Testnet.Chain(), flowTokenAccountIndex),
	}
	contractAddressesByChainID[flow.Testnet] = testnet

	// Stagingnet test network
	// All system contracts are deployed to the service account
	stagingnet := map[string]flow.Address{
		ContractNameEpoch:         flow.Stagingnet.Chain().ServiceAddress(),
		ContractNameClusterQC:     flow.Stagingnet.Chain().ServiceAddress(),
		ContractNameDKG:           flow.Stagingnet.Chain().ServiceAddress(),
		ContractNameFlowFees:      mustAddressAtIndex(flow.Stagingnet.Chain(), flowFeesAccountIndex),
		ContractNameFungibleToken: mustAddressAtIndex(flow.Stagingnet.Chain(), fungibleTokenAccountIndex),
		ContractNameFlowToken:     mustAddressAtIndex(flow.Stagingnet.Chain(), flowTokenAccountIndex),
	}
	contractAddressesByChainID[flow.Stagingnet] = stagingnet

	// Transient test networks
	// All system contracts are deployed to the service account
	transient := map[string]flow.Address{
		ContractNameEpoch:         flow.Emulator.Chain().ServiceAddress(),
		ContractNameClusterQC:     flow.Emulator.Chain().ServiceAddress(),
		ContractNameDKG:           flow.Emulator.Chain().ServiceAddress(),
		ContractNameFlowFees:      flow.Emulator.Chain().ServiceAddress(),
		ContractNameFungibleToken: flow.Emulator.Chain().ServiceAddress(),
		ContractNameFlowToken:     flow.Emulator.Chain().ServiceAddress(),
	}
	contractAddressesByChainID[flow.Emulator] = transient
	contractAddressesByChainID[flow.MonotonicEmulator] = transient
	contractAddressesByChainID[flow.Localnet] = transient
	contractAddressesByChainID[flow.BftTestnet] = transient
	contractAddressesByChainID[flow.Benchnet] = transient

}

func mustAddressAtIndex(chain flow.Chain, index uint64) flow.Address {
	address, err := chain.AddressAtIndex(fungibleTokenAccountIndex)
	if err != nil {
		// this should never panic as the index should always be ok
		panic(err)
	}
	return address
}
