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

	"github.com/onflow/flow-core-contracts/lib/go/templates"

	"github.com/onflow/flow-go/model/flow"
)

const (
	// Unqualified names of system smart contracts (not including address prefix)

	ContractNameEpoch               = "FlowEpoch"
	ContractNameIDTableStaking      = "FlowIDTableStaking"
	ContractNameClusterQC           = "FlowClusterQC"
	ContractNameDKG                 = "FlowDKG"
	ContractNameServiceAccount      = "FlowServiceAccount"
	ContractNameFlowFees            = "FlowFees"
	ContractNameStorageFees         = "FlowStorageFees"
	ContractNameNodeVersionBeacon   = "NodeVersionBeacon"
	ContractNameRandomBeaconHistory = "RandomBeaconHistory"
	ContractNameFungibleToken       = "FungibleToken"
	ContractNameFlowToken           = "FlowToken"
	ContractNameNonFungibleToken    = "NonFungibleToken"
	ContractNameMetadataViews       = "MetadataViews"
	ContractNameViewResolver        = "ViewResolver"
	ContractNameEVM                 = "EVM"

	// Unqualified names of service events (not including address prefix or contract name)

	EventNameEpochSetup    = "EpochSetup"
	EventNameEpochCommit   = "EpochCommit"
	EventNameVersionBeacon = "VersionBeacon"

	//  Unqualified names of service event contract functions (not including address prefix or contract name)

	ContractServiceAccountFunction_setupNewAccount                            = "setupNewAccount"
	ContractServiceAccountFunction_defaultTokenBalance                        = "defaultTokenBalance"
	ContractServiceAccountFunction_deductTransactionFee                       = "deductTransactionFee"
	ContractServiceAccountFunction_verifyPayersBalanceForTransactionExecution = "verifyPayersBalanceForTransactionExecution"
	ContractStorageFeesFunction_calculateAccountCapacity                      = "calculateAccountCapacity"
	ContractStorageFeesFunction_getAccountsCapacityForTransactionStorageCheck = "getAccountsCapacityForTransactionStorageCheck"
	ContractStorageFeesFunction_defaultTokenAvailableBalance                  = "defaultTokenAvailableBalance"

	// Indexes of the system contracts that are deployed to an address at a specific index

	FungibleTokenAccountIndex = 2
	FlowTokenAccountIndex     = 3
	FlowFeesAccountIndex      = 4
	EVMAccountIndex           = 5
)

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

	// nftTokenAddressTestnet is the address of the NonFungibleToken contract on Testnet
	nftTokenAddressMainnet = flow.HexToAddress("1d7e57aa55817448")
	// nftTokenAddressTestnet is the address of the NonFungibleToken contract on Testnet
	nftTokenAddressTestnet = flow.HexToAddress("631e88ae7f1d7c20")
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
	// epoch related contracts
	Epoch          SystemContract
	IDTableStaking SystemContract
	ClusterQC      SystemContract
	DKG            SystemContract

	// service account related contracts
	FlowServiceAccount  SystemContract
	NodeVersionBeacon   SystemContract
	RandomBeaconHistory SystemContract
	FlowStorageFees     SystemContract

	// token related contracts
	FlowFees      SystemContract
	FlowToken     SystemContract
	FungibleToken SystemContract

	// NFT related contracts
	NonFungibleToken SystemContract
	MetadataViews    SystemContract
	ViewResolver     SystemContract

	// EVM related contracts
	EVM SystemContract
}

// AsTemplateEnv returns a template environment with all system contracts filled in.
// This is useful for generating Cadence code from templates.
func (c SystemContracts) AsTemplateEnv() templates.Environment {
	return templates.Environment{
		EpochAddress:             c.Epoch.Address.Hex(),
		IDTableAddress:           c.IDTableStaking.Address.Hex(),
		QuorumCertificateAddress: c.ClusterQC.Address.Hex(),
		DkgAddress:               c.DKG.Address.Hex(),

		ServiceAccountAddress:      c.FlowServiceAccount.Address.Hex(),
		NodeVersionBeaconAddress:   c.NodeVersionBeacon.Address.Hex(),
		RandomBeaconHistoryAddress: c.RandomBeaconHistory.Address.Hex(),
		StorageFeesAddress:         c.FlowStorageFees.Address.Hex(),

		FlowFeesAddress:      c.FlowFees.Address.Hex(),
		FlowTokenAddress:     c.FlowToken.Address.Hex(),
		FungibleTokenAddress: c.FungibleToken.Address.Hex(),

		// NonFungibleToken: c.NonFungibleToken.Address.Hex(),
		// MetadataViews : c.MetadataViews.Address.Hex(),
		// ViewResolver : c.ViewResolver.Address.Hex(),

		// EVMAddress: c.EVM.Address.Hex(), // doesn't exist yet
	}
}

// All returns all system contracts as a slice.
func (c SystemContracts) All() []SystemContract {
	return []SystemContract{
		c.Epoch,
		c.IDTableStaking,
		c.ClusterQC,
		c.DKG,

		c.FlowServiceAccount,
		c.NodeVersionBeacon,
		c.RandomBeaconHistory,
		c.FlowStorageFees,

		c.FlowFees,
		c.FlowToken,
		c.FungibleToken,

		c.NonFungibleToken,
		c.MetadataViews,
		c.ViewResolver,

		c.EVM,
	}
}

// ServiceEvents is a container for all service events on a particular chain.
type ServiceEvents struct {
	EpochSetup    ServiceEvent
	EpochCommit   ServiceEvent
	VersionBeacon ServiceEvent
}

// All returns all service events as a slice.
func (se ServiceEvents) All() []ServiceEvent {
	return []ServiceEvent{
		se.EpochSetup,
		se.EpochCommit,
		se.VersionBeacon,
	}
}

// SystemContractsForChain returns the system contract configuration for the given chain.
// Panics if the chain is unknown.
func SystemContractsForChain(chainID flow.ChainID) *SystemContracts {
	contracts, ok := systemContractsForChain[chainID]
	if !ok {
		// this is a panic, since it can only happen if the code is wrong
		panic(fmt.Sprintf("unknown chain: %s", chainID))
	}
	return contracts
}

var systemContractsForChain = map[flow.ChainID]*SystemContracts{}

// ServiceEventsForChain returns the service event confirmation for the given chain.
// Panics if the chain is unknown.
func ServiceEventsForChain(chainID flow.ChainID) *ServiceEvents {
	events, ok := serviceEventsForChain[chainID]
	if !ok {
		// this is a panic, since it can only happen if the code is wrong
		panic(fmt.Sprintf("unknown chain: %s", chainID))
	}
	return events
}

var serviceEventsForChain = map[flow.ChainID]*ServiceEvents{}

var contractAddressFunc = map[string]func(id flow.ChainID) flow.Address{}

func init() {

	serviceAddressFunc := func(chain flow.ChainID) flow.Address {
		return chain.Chain().ServiceAddress()
	}

	// epoch contracts are deployed on a separate account on mainnet and testnet
	epochAddressFunc := func(chain flow.ChainID) flow.Address {
		switch chain {
		case flow.Mainnet:
			return stakingContractAddressMainnet
		case flow.Testnet:
			return stakingContractAddressTestnet
		default:
			return chain.Chain().ServiceAddress()
		}
	}

	// some contracts are always at an address with a a predetermined index
	nthAddressFunc := func(index uint64) func(chain flow.ChainID) flow.Address {
		return func(chain flow.ChainID) flow.Address {
			address, err := chain.Chain().AddressAtIndex(index)
			if err != nil {
				// this can only happen if the code is wrong
				panic(fmt.Sprintf("failed to get %d address: %v", FlowFeesAccountIndex, err))
			}
			return address
		}
	}

	nftTokenAddressFunc := func(chain flow.ChainID) flow.Address {
		switch chain {
		case flow.Mainnet:
			return nftTokenAddressMainnet
		case flow.Testnet:
			return nftTokenAddressTestnet
		default:
			return chain.Chain().ServiceAddress()
		}
	}

	contractAddressFunc = map[string]func(id flow.ChainID) flow.Address{
		ContractNameIDTableStaking: epochAddressFunc,
		ContractNameEpoch:          epochAddressFunc,
		ContractNameClusterQC:      epochAddressFunc,
		ContractNameDKG:            epochAddressFunc,

		ContractNameNodeVersionBeacon:   serviceAddressFunc,
		ContractNameRandomBeaconHistory: serviceAddressFunc,
		ContractNameServiceAccount:      serviceAddressFunc,
		ContractNameStorageFees:         serviceAddressFunc,

		ContractNameFlowFees:      nthAddressFunc(FlowFeesAccountIndex),
		ContractNameFungibleToken: nthAddressFunc(FungibleTokenAccountIndex),
		ContractNameFlowToken:     nthAddressFunc(FlowTokenAccountIndex),

		ContractNameNonFungibleToken: nftTokenAddressFunc,
		ContractNameMetadataViews:    nftTokenAddressFunc,
		ContractNameViewResolver:     nftTokenAddressFunc,

		ContractNameEVM: nthAddressFunc(EVMAccountIndex),
	}

	getSystemContractsForChain := func(chainID flow.ChainID) *SystemContracts {

		contract := func(name string) SystemContract {
			addressFunc, ok := contractAddressFunc[name]
			if !ok {
				// this is a panic, since it can only happen if the code is wrong
				panic(fmt.Sprintf("unknown system contract name: %s", name))
			}

			return SystemContract{
				Address: addressFunc(chainID),
				Name:    name,
			}
		}

		contracts := &SystemContracts{
			Epoch:          contract(ContractNameEpoch),
			IDTableStaking: contract(ContractNameIDTableStaking),
			ClusterQC:      contract(ContractNameClusterQC),
			DKG:            contract(ContractNameDKG),

			FlowServiceAccount:  contract(ContractNameServiceAccount),
			NodeVersionBeacon:   contract(ContractNameNodeVersionBeacon),
			RandomBeaconHistory: contract(ContractNameRandomBeaconHistory),
			FlowStorageFees:     contract(ContractNameStorageFees),

			FlowFees:      contract(ContractNameFlowFees),
			FlowToken:     contract(ContractNameFlowToken),
			FungibleToken: contract(ContractNameFungibleToken),

			NonFungibleToken: contract(ContractNameNonFungibleToken),
			MetadataViews:    contract(ContractNameMetadataViews),
			ViewResolver:     contract(ContractNameViewResolver),

			EVM: contract(ContractNameEVM),
		}

		return contracts
	}

	getServiceEventsForChain := func(chainID flow.ChainID) *ServiceEvents {

		event := func(contractName, eventName string) ServiceEvent {
			addressFunc, ok := contractAddressFunc[contractName]
			if !ok {
				// this is a panic, since it can only happen if the code is wrong
				panic(fmt.Sprintf("unknown system contract name: %s", contractName))
			}

			return ServiceEvent{
				Address:      addressFunc(chainID),
				ContractName: contractName,
				Name:         eventName,
			}
		}

		events := &ServiceEvents{
			EpochSetup:    event(ContractNameEpoch, EventNameEpochSetup),
			EpochCommit:   event(ContractNameEpoch, EventNameEpochCommit),
			VersionBeacon: event(ContractNameNodeVersionBeacon, EventNameVersionBeacon),
		}

		return events
	}

	// pre-populate the system contracts and service events for all chains for fast access
	for _, chain := range flow.AllChainIDs() {
		serviceEventsForChain[chain] = getServiceEventsForChain(chain)
		systemContractsForChain[chain] = getSystemContractsForChain(chain)
	}
}
