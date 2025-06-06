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

	"github.com/onflow/cadence/common"
	"github.com/onflow/flow-core-contracts/lib/go/templates"

	"github.com/onflow/flow-go/model/flow"
)

const (
	// Unqualified names of system smart contracts (not including address prefix)

	ContractNameEpoch                      = "FlowEpoch"
	ContractNameIDTableStaking             = "FlowIDTableStaking"
	ContractNameClusterQC                  = "FlowClusterQC"
	ContractNameDKG                        = "FlowDKG"
	ContractNameServiceAccount             = "FlowServiceAccount"
	ContractNameFlowFees                   = "FlowFees"
	ContractNameStorageFees                = "FlowStorageFees"
	ContractNameNodeVersionBeacon          = "NodeVersionBeacon"
	ContractNameRandomBeaconHistory        = "RandomBeaconHistory"
	ContractNameFungibleToken              = "FungibleToken"
	ContractNameFlowToken                  = "FlowToken"
	ContractNameFungibleTokenSwitchboard   = "FungibleTokenSwitchboard"
	ContractNameFungibleTokenMetadataViews = "FungibleTokenMetadataViews"
	ContractNameNonFungibleToken           = "NonFungibleToken"
	ContractNameMetadataViews              = "MetadataViews"
	ContractNameViewResolver               = "ViewResolver"
	ContractNameCrossVMMetadataViews       = "CrossVMMetadataViews"
	ContractNameEVM                        = "EVM"
	ContractNameBurner                     = "Burner"
	ContractNameCrypto                     = "Crypto"
	ContractNameMigration                  = "Migration"

	// AccountNameEVMStorage is not a contract, but a special account that is used to store EVM state
	AccountNameEVMStorage = "EVMStorageAccount"
	// AccountNameExecutionParametersAccount is not a contract, but a special account that is used to store execution parameters
	// It is a separate account on all networks in order to separate it away
	// from the frequently changing data on the service account.
	AccountNameExecutionParametersAccount = "ExecutionParametersAccount"

	// Unqualified names of service events (not including address prefix or contract name)

	EventNameEpochSetup                  = "EpochSetup"
	EventNameEpochCommit                 = "EpochCommit"
	EventNameEpochRecover                = "EpochRecover"
	EventNameVersionBeacon               = "VersionBeacon"               // VersionBeacon only controls version of ENs, describing software compatability via semantic versioning
	EventNameProtocolStateVersionUpgrade = "ProtocolStateVersionUpgrade" // Protocol State version applies to all nodes and uses an _integer version_ of the _protocol state_

	//  Unqualified names of service event contract functions (not including address prefix or contract name)

	ContractServiceAccountFunction_setupNewAccount                            = "setupNewAccount"
	ContractServiceAccountFunction_defaultTokenBalance                        = "defaultTokenBalance"
	ContractServiceAccountFunction_deductTransactionFee                       = "deductTransactionFee"
	ContractServiceAccountFunction_verifyPayersBalanceForTransactionExecution = "verifyPayersBalanceForTransactionExecution"
	ContractStorageFeesFunction_calculateAccountCapacity                      = "calculateAccountCapacity"
	ContractStorageFeesFunction_getAccountsCapacityForTransactionStorageCheck = "getAccountsCapacityForTransactionStorageCheck"
	ContractStorageFeesFunction_defaultTokenAvailableBalance                  = "defaultTokenAvailableBalance"

	// These are the account indexes of system contracts as deployed by the default bootstrapping.
	// On long-running networks some of these contracts might have been deployed after bootstrapping,
	// and therefore might not be at these indexes.

	FungibleTokenAccountIndex = 2
	FlowTokenAccountIndex     = 3
	FlowFeesAccountIndex      = 4
	EVMStorageAccountIndex    = 5

	// LastSystemAccountIndex is the last index of a system accounts.
	// Other addresses will be created  after this one.
	LastSystemAccountIndex = EVMStorageAccountIndex
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

	// evmStorageAddressTestnet is the address of the EVM state storage contract on Testnet
	evmStorageAddressTestnet = flow.HexToAddress("1a54ed2be7552821")
	// evmStorageAddressMainnet is the address of the EVM state storage contract on Mainnet
	evmStorageAddressMainnet = flow.HexToAddress("d421a63faae318f9")

	// executionParametersAddressTestnet is the address of the Execution Parameters contract on Testnet
	executionParametersAddressTestnet = flow.HexToAddress("6997a2f2cf57b73a")
	// executionParametersAddressMainnet is the address of the Execution Parameters contract on Mainnet
	executionParametersAddressMainnet = flow.HexToAddress("f426ff57ee8f6110")
)

// SystemContract represents a system contract on a particular chain.
type SystemContract struct {
	Address flow.Address
	Name    string
}

func (c SystemContract) Location() common.AddressLocation {
	return common.AddressLocation{
		Address: common.Address(c.Address),
		Name:    c.Name,
	}
}

// SystemAccount represents an address used by the system.
type SystemAccount struct {
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
	FlowServiceAccount         SystemContract
	NodeVersionBeacon          SystemContract
	RandomBeaconHistory        SystemContract
	FlowStorageFees            SystemContract
	ExecutionParametersAccount SystemContract

	// token related contracts
	FlowFees                   SystemContract
	FlowToken                  SystemContract
	FungibleToken              SystemContract
	FungibleTokenSwitchboard   SystemContract
	FungibleTokenMetadataViews SystemContract

	// NFT related contracts
	NonFungibleToken     SystemContract
	MetadataViews        SystemContract
	ViewResolver         SystemContract
	CrossVMMetadataViews SystemContract

	// EVM related contracts
	EVMContract SystemContract
	EVMStorage  SystemAccount

	// Utility contracts
	Burner SystemContract
	Crypto SystemContract

	// Migration contracts
	Migration SystemContract
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
		EVMAddress:                 c.EVMContract.Address.Hex(),

		FlowFeesAddress:                   c.FlowFees.Address.Hex(),
		FlowTokenAddress:                  c.FlowToken.Address.Hex(),
		FungibleTokenAddress:              c.FungibleToken.Address.Hex(),
		FungibleTokenSwitchboardAddress:   c.FungibleTokenSwitchboard.Address.Hex(),
		FungibleTokenMetadataViewsAddress: c.FungibleTokenMetadataViews.Address.Hex(),

		NonFungibleTokenAddress:     c.NonFungibleToken.Address.Hex(),
		MetadataViewsAddress:        c.MetadataViews.Address.Hex(),
		CrossVMMetadataViewsAddress: c.CrossVMMetadataViews.Address.Hex(),
		ViewResolverAddress:         c.ViewResolver.Address.Hex(),

		BurnerAddress: c.Burner.Address.Hex(),
		CryptoAddress: c.Crypto.Address.Hex(),
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
		c.FungibleTokenMetadataViews,
		c.FungibleTokenSwitchboard,

		c.NonFungibleToken,
		c.MetadataViews,
		c.ViewResolver,
		c.CrossVMMetadataViews,

		c.EVMContract,
		// EVMStorage is not included here, since it is not a contract

		c.Burner,
		c.Crypto,

		c.Migration,
	}
}

// ServiceEvents is a container for all service events on a particular chain.
type ServiceEvents struct {
	EpochSetup                  ServiceEvent
	EpochCommit                 ServiceEvent
	EpochRecover                ServiceEvent
	VersionBeacon               ServiceEvent
	ProtocolStateVersionUpgrade ServiceEvent
}

// All returns all service events as a slice.
func (se ServiceEvents) All() []ServiceEvent {
	return []ServiceEvent{
		se.EpochSetup,
		se.EpochCommit,
		se.EpochRecover,
		se.VersionBeacon,
		se.ProtocolStateVersionUpgrade,
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

	evmStorageEVMFunc := func(chain flow.ChainID) flow.Address {
		switch chain {
		case flow.Mainnet:
			return evmStorageAddressMainnet
		case flow.Testnet:
			return evmStorageAddressTestnet
		default:
			return nthAddressFunc(EVMStorageAccountIndex)(chain)
		}
	}

	burnerAddressFunc := func(chain flow.ChainID) flow.Address {
		switch chain {
		case flow.Mainnet, flow.Testnet:
			return nthAddressFunc(FungibleTokenAccountIndex)(chain)
		default:
			return serviceAddressFunc(chain)
		}
	}

	executionParametersAccountFunc := func(chain flow.ChainID) flow.Address {
		switch chain {
		case flow.Mainnet:
			return executionParametersAddressMainnet
		case flow.Testnet:
			return executionParametersAddressTestnet
		default:
			return nthAddressFunc(FungibleTokenAccountIndex)(chain)
		}
	}

	contractAddressFunc = map[string]func(id flow.ChainID) flow.Address{
		ContractNameIDTableStaking: epochAddressFunc,
		ContractNameEpoch:          epochAddressFunc,
		ContractNameClusterQC:      epochAddressFunc,
		ContractNameDKG:            epochAddressFunc,

		ContractNameNodeVersionBeacon:         serviceAddressFunc,
		ContractNameRandomBeaconHistory:       serviceAddressFunc,
		ContractNameServiceAccount:            serviceAddressFunc,
		ContractNameStorageFees:               serviceAddressFunc,
		AccountNameExecutionParametersAccount: executionParametersAccountFunc,

		ContractNameFlowFees:                   nthAddressFunc(FlowFeesAccountIndex),
		ContractNameFungibleToken:              nthAddressFunc(FungibleTokenAccountIndex),
		ContractNameFlowToken:                  nthAddressFunc(FlowTokenAccountIndex),
		ContractNameFungibleTokenSwitchboard:   nthAddressFunc(FungibleTokenAccountIndex),
		ContractNameFungibleTokenMetadataViews: nthAddressFunc(FungibleTokenAccountIndex),

		ContractNameNonFungibleToken:     nftTokenAddressFunc,
		ContractNameMetadataViews:        nftTokenAddressFunc,
		ContractNameViewResolver:         nftTokenAddressFunc,
		ContractNameCrossVMMetadataViews: nftTokenAddressFunc,

		ContractNameEVM:       serviceAddressFunc,
		AccountNameEVMStorage: evmStorageEVMFunc,

		ContractNameBurner: burnerAddressFunc,
		ContractNameCrypto: serviceAddressFunc,

		ContractNameMigration: serviceAddressFunc,
	}

	getSystemContractsForChain := func(chainID flow.ChainID) *SystemContracts {

		addressOfContract := func(name string) SystemContract {
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

		addressOfAccount := func(name string) SystemAccount {
			addressFunc, ok := contractAddressFunc[name]
			if !ok {
				// this is a panic, since it can only happen if the code is wrong
				panic(fmt.Sprintf("unknown system account name: %s", name))
			}

			return SystemAccount{
				Address: addressFunc(chainID),
				Name:    name,
			}
		}

		contracts := &SystemContracts{
			Epoch:          addressOfContract(ContractNameEpoch),
			IDTableStaking: addressOfContract(ContractNameIDTableStaking),
			ClusterQC:      addressOfContract(ContractNameClusterQC),
			DKG:            addressOfContract(ContractNameDKG),

			FlowServiceAccount:         addressOfContract(ContractNameServiceAccount),
			NodeVersionBeacon:          addressOfContract(ContractNameNodeVersionBeacon),
			RandomBeaconHistory:        addressOfContract(ContractNameRandomBeaconHistory),
			FlowStorageFees:            addressOfContract(ContractNameStorageFees),
			ExecutionParametersAccount: addressOfContract(AccountNameExecutionParametersAccount),

			FlowFees:                   addressOfContract(ContractNameFlowFees),
			FlowToken:                  addressOfContract(ContractNameFlowToken),
			FungibleToken:              addressOfContract(ContractNameFungibleToken),
			FungibleTokenMetadataViews: addressOfContract(ContractNameFungibleTokenMetadataViews),
			FungibleTokenSwitchboard:   addressOfContract(ContractNameFungibleTokenSwitchboard),

			NonFungibleToken:     addressOfContract(ContractNameNonFungibleToken),
			MetadataViews:        addressOfContract(ContractNameMetadataViews),
			ViewResolver:         addressOfContract(ContractNameViewResolver),
			CrossVMMetadataViews: addressOfContract(ContractNameCrossVMMetadataViews),

			EVMContract: addressOfContract(ContractNameEVM),
			EVMStorage:  addressOfAccount(AccountNameEVMStorage),

			Burner: addressOfContract(ContractNameBurner),
			Crypto: addressOfContract(ContractNameCrypto),

			Migration: addressOfContract(ContractNameMigration),
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
			EpochSetup:                  event(ContractNameEpoch, EventNameEpochSetup),
			EpochCommit:                 event(ContractNameEpoch, EventNameEpochCommit),
			EpochRecover:                event(ContractNameEpoch, EventNameEpochRecover),
			VersionBeacon:               event(ContractNameNodeVersionBeacon, EventNameVersionBeacon),
			ProtocolStateVersionUpgrade: event(ContractNameNodeVersionBeacon, EventNameProtocolStateVersionUpgrade),
		}
		return events
	}

	// pre-populate the system contracts and service events for all chains for fast access
	for _, chain := range flow.AllChainIDs() {
		serviceEventsForChain[chain] = getServiceEventsForChain(chain)
		systemContractsForChain[chain] = getSystemContractsForChain(chain)
	}
}
