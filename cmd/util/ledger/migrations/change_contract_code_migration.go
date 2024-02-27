package migrations

import (
	"fmt"

	coreContracts "github.com/onflow/flow-core-contracts/lib/go/contracts"
	ftContracts "github.com/onflow/flow-ft/lib/go/contracts"
	nftContracts "github.com/onflow/flow-nft/lib/go/contracts"

	sdk "github.com/onflow/flow-go-sdk"

	"github.com/onflow/cadence/runtime/common"

	evm "github.com/onflow/flow-go/fvm/evm/stdlib"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
)

type ChangeContractCodeMigration struct {
	*StagedContractsMigration
}

var _ AccountBasedMigration = (*ChangeContractCodeMigration)(nil)

func NewChangeContractCodeMigration(chainID flow.ChainID) *ChangeContractCodeMigration {
	return &ChangeContractCodeMigration{
		StagedContractsMigration: NewStagedContractsMigration(chainID).
			// TODO:
			//WithContractUpdateValidation().
			WithName("ChangeContractCodeMigration"),
	}
}

func NewSystemContractChange(
	systemContract systemcontracts.SystemContract,
	newContractCode []byte,
) StagedContract {
	return StagedContract{
		Address: common.Address(systemContract.Address),
		Contract: Contract{
			Name: systemContract.Name,
			Code: newContractCode,
		},
	}
}

type EVMContractChange uint8

const (
	EVMContractChangeNone EVMContractChange = iota
	EVMContractChangeABIOnly
	EVMContractChangeFull
)

type SystemContractChangesOptions struct {
	EVM EVMContractChange
}

func BurnerAddressForChain(chainID flow.ChainID) flow.Address {

	systemContracts := systemcontracts.SystemContractsForChain(chainID)
	serviceAccountAddress := systemContracts.FlowServiceAccount.Address
	fungibleTokenAddress := systemContracts.FungibleToken.Address

	switch chainID {
	case flow.Mainnet, flow.Testnet:
		return fungibleTokenAddress

	case flow.Emulator, flow.Localnet:
		return serviceAccountAddress

	default:
		panic(fmt.Errorf("unsupported chain ID: %s", chainID))
	}
}

func SystemContractChanges(chainID flow.ChainID, options SystemContractChangesOptions) []StagedContract {
	systemContracts := systemcontracts.SystemContractsForChain(chainID)

	serviceAccountAddress := systemContracts.FlowServiceAccount.Address
	idTableStakingAddress := systemContracts.IDTableStaking.Address
	clusterQCAddress := systemContracts.ClusterQC.Address
	dkgAddress := systemContracts.DKG.Address
	fungibleTokenAddress := systemContracts.FungibleToken.Address
	flowTokenAddress := systemContracts.FlowToken.Address
	flowFeesAddress := systemContracts.FlowFees.Address
	flowStorageFeesAddress := systemContracts.FlowStorageFees.Address
	viewResolverAddress := systemContracts.ViewResolver.Address
	metadataViewsAddress := systemContracts.MetadataViews.Address
	fungibleTokenMetadataViewsAddress := common.Address(fungibleTokenAddress)
	fungibleTokenSwitchboardAddress := common.Address(fungibleTokenAddress)

	burnerAddress := BurnerAddressForChain(chainID)

	var stakingCollectionAddress common.Address
	var stakingProxyAddress common.Address

	switch chainID {
	case flow.Mainnet:
		stakingCollectionAddress = mustHexAddress("0x8d0e87b65159ae63")
		stakingProxyAddress = mustHexAddress("0x62430cf28c26d095")

	case flow.Testnet:
		stakingCollectionAddress = mustHexAddress("0x95e019a17d0e23d7")
		stakingProxyAddress = mustHexAddress("0x7aad92e5a0715d21")

	case flow.Emulator, flow.Localnet:
		stakingCollectionAddress = common.Address(serviceAccountAddress)
		stakingProxyAddress = common.Address(serviceAccountAddress)

	default:
		panic(fmt.Errorf("unsupported chain ID: %s", chainID))
	}

	lockedTokensAddress := stakingCollectionAddress

	contractChanges := []StagedContract{
		// epoch related contracts
		NewSystemContractChange(
			systemContracts.Epoch,
			coreContracts.FlowEpoch(
				fungibleTokenAddress.HexWithPrefix(),
				flowTokenAddress.HexWithPrefix(),
				idTableStakingAddress.HexWithPrefix(),
				clusterQCAddress.HexWithPrefix(),
				dkgAddress.HexWithPrefix(),
				flowFeesAddress.HexWithPrefix(),
			),
		),
		NewSystemContractChange(
			systemContracts.IDTableStaking,
			coreContracts.FlowIDTableStaking(
				fungibleTokenAddress.HexWithPrefix(),
				flowTokenAddress.HexWithPrefix(),
				flowFeesAddress.HexWithPrefix(),
				burnerAddress.HexWithPrefix(),
				true,
			),
		),
		NewSystemContractChange(
			systemContracts.ClusterQC,
			coreContracts.FlowQC(),
		),
		NewSystemContractChange(
			systemContracts.DKG,
			coreContracts.FlowDKG(),
		),

		// service account related contracts
		NewSystemContractChange(
			systemContracts.FlowServiceAccount,
			coreContracts.FlowServiceAccount(
				fungibleTokenAddress.HexWithPrefix(),
				flowTokenAddress.HexWithPrefix(),
				flowFeesAddress.HexWithPrefix(),
				flowStorageFeesAddress.HexWithPrefix(),
			),
		),
		NewSystemContractChange(
			systemContracts.NodeVersionBeacon,
			coreContracts.NodeVersionBeacon(),
		),
		NewSystemContractChange(
			systemContracts.RandomBeaconHistory,
			coreContracts.RandomBeaconHistory(),
		),
		NewSystemContractChange(
			systemContracts.FlowStorageFees,
			coreContracts.FlowStorageFees(
				fungibleTokenAddress.HexWithPrefix(),
				flowTokenAddress.HexWithPrefix(),
			),
		),
		{
			Address: stakingCollectionAddress,
			Contract: Contract{
				Name: "FlowStakingCollection",
				Code: coreContracts.FlowStakingCollection(
					fungibleTokenAddress.HexWithPrefix(),
					flowTokenAddress.HexWithPrefix(),
					idTableStakingAddress.HexWithPrefix(),
					stakingProxyAddress.HexWithPrefix(),
					lockedTokensAddress.HexWithPrefix(),
					flowStorageFeesAddress.HexWithPrefix(),
					clusterQCAddress.HexWithPrefix(),
					dkgAddress.HexWithPrefix(),
					systemContracts.Epoch.Address.HexWithPrefix(),
				),
			},
		},
		{
			Address: stakingProxyAddress,
			Contract: Contract{
				Name: "StakingProxy",
				Code: coreContracts.FlowStakingProxy(),
			},
		},
		{
			Address: lockedTokensAddress,
			Contract: Contract{
				Name: "LockedTokens",
				Code: coreContracts.FlowLockedTokens(
					fungibleTokenAddress.HexWithPrefix(),
					flowTokenAddress.HexWithPrefix(),
					idTableStakingAddress.HexWithPrefix(),
					stakingProxyAddress.HexWithPrefix(),
					flowStorageFeesAddress.HexWithPrefix(),
				),
			},
		},

		// token related contracts
		NewSystemContractChange(
			systemContracts.FlowFees,
			coreContracts.FlowFees(
				fungibleTokenAddress.HexWithPrefix(),
				flowTokenAddress.HexWithPrefix(),
				flowStorageFeesAddress.HexWithPrefix(),
			),
		),
		NewSystemContractChange(
			systemContracts.FlowToken,
			coreContracts.FlowToken(
				fungibleTokenAddress.HexWithPrefix(),
				fungibleTokenMetadataViewsAddress.HexWithPrefix(),
				metadataViewsAddress.HexWithPrefix(),
				burnerAddress.HexWithPrefix(),
			),
		),
		NewSystemContractChange(
			systemContracts.FungibleToken,
			ftContracts.FungibleToken(
				// Use `Hex()`, since this method adds the prefix.
				viewResolverAddress.Hex(),
				burnerAddress.Hex(),
			),
		),
		{
			Address: fungibleTokenMetadataViewsAddress,
			Contract: Contract{
				Name: "FungibleTokenMetadataViews",
				Code: ftContracts.FungibleTokenMetadataViews(
					// Use `Hex()`, since this method adds the prefix.
					fungibleTokenAddress.Hex(),
					metadataViewsAddress.Hex(),
					viewResolverAddress.Hex(),
				),
			},
		},

		// NFT related contracts
		NewSystemContractChange(
			systemContracts.NonFungibleToken,
			nftContracts.NonFungibleToken(
				sdk.Address(viewResolverAddress),
			),
		),
		NewSystemContractChange(
			systemContracts.MetadataViews,
			nftContracts.MetadataViews(
				sdk.Address(fungibleTokenAddress),
				sdk.Address(systemContracts.NonFungibleToken.Address),
				sdk.Address(viewResolverAddress),
			),
		),
		NewSystemContractChange(
			systemContracts.ViewResolver,
			nftContracts.ViewResolver(),
		),
	}

	switch chainID {
	case flow.Emulator, flow.Localnet:
		// skip

	default:
		contractChanges = append(
			contractChanges,
			StagedContract{
				Address: fungibleTokenSwitchboardAddress,
				Contract: Contract{
					Name: "FungibleTokenSwitchboard",
					Code: ftContracts.FungibleTokenSwitchboard(
						fungibleTokenAddress.HexWithPrefix(),
					),
				},
			},
		)
	}

	// EVM related contracts
	switch options.EVM {
	case EVMContractChangeNone:
		// do nothing
	case EVMContractChangeABIOnly, EVMContractChangeFull:
		abiOnly := options.EVM == EVMContractChangeABIOnly
		contractChanges = append(
			contractChanges,
			NewSystemContractChange(
				systemContracts.EVMContract,
				evm.ContractCode(
					flowTokenAddress,
					abiOnly,
				),
			),
		)
	default:
		panic(fmt.Errorf("unsupported EVM contract change option: %d", options.EVM))
	}

	return contractChanges
}

func mustHexAddress(hexAddress string) common.Address {
	address, err := common.HexToAddress(hexAddress)
	if err != nil {
		panic(err)
	}
	return address
}
