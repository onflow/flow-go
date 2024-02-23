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

func SystemContractChanges(chainID flow.ChainID, options SystemContractChangesOptions) []StagedContract {
	systemContracts := systemcontracts.SystemContractsForChain(chainID)

	var stakingCollectionAddress, stakingProxyAddress common.Address

	switch chainID {
	case flow.Mainnet:
		stakingCollectionAddress = mustHexAddress("0x8d0e87b65159ae63")
		stakingProxyAddress = mustHexAddress("0x62430cf28c26d095")

	case flow.Testnet:
		stakingCollectionAddress = mustHexAddress("0x95e019a17d0e23d7")
		stakingProxyAddress = mustHexAddress("0x7aad92e5a0715d21")

	case flow.Emulator:
		stakingCollectionAddress = common.Address(systemContracts.FlowServiceAccount.Address)
		stakingProxyAddress = common.Address(systemContracts.FlowServiceAccount.Address)

	default:
		panic(fmt.Errorf("unsupported chain ID: %s", chainID))
	}

	lockedTokensAddress := stakingCollectionAddress
	fungibleTokenMetadataViewsAddress := common.Address(systemContracts.FungibleToken.Address)
	fungibleTokenSwitchboardAddress := common.Address(systemContracts.FungibleToken.Address)

	contractChanges := []StagedContract{
		// epoch related contracts
		NewSystemContractChange(
			systemContracts.Epoch,
			coreContracts.FlowEpoch(
				systemContracts.FungibleToken.Address.HexWithPrefix(),
				systemContracts.FlowToken.Address.HexWithPrefix(),
				systemContracts.IDTableStaking.Address.HexWithPrefix(),
				systemContracts.ClusterQC.Address.HexWithPrefix(),
				systemContracts.DKG.Address.HexWithPrefix(),
				systemContracts.FlowFees.Address.HexWithPrefix(),
			),
		),
		NewSystemContractChange(
			systemContracts.IDTableStaking,
			coreContracts.FlowIDTableStaking(
				systemContracts.FungibleToken.Address.HexWithPrefix(),
				systemContracts.FlowToken.Address.HexWithPrefix(),
				systemContracts.FlowFees.Address.HexWithPrefix(),
				systemContracts.FlowServiceAccount.Address.HexWithPrefix(),
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
				systemContracts.FungibleToken.Address.HexWithPrefix(),
				systemContracts.FlowToken.Address.HexWithPrefix(),
				systemContracts.FlowFees.Address.HexWithPrefix(),
				systemContracts.FlowStorageFees.Address.HexWithPrefix(),
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
				systemContracts.FungibleToken.Address.HexWithPrefix(),
				systemContracts.FlowToken.Address.HexWithPrefix(),
			),
		),
		{
			Address: stakingCollectionAddress,
			Contract: Contract{
				Name: "FlowStakingCollection",
				Code: coreContracts.FlowStakingCollection(
					systemContracts.FungibleToken.Address.HexWithPrefix(),
					systemContracts.FlowToken.Address.HexWithPrefix(),
					systemContracts.IDTableStaking.Address.HexWithPrefix(),
					stakingProxyAddress.HexWithPrefix(),
					lockedTokensAddress.HexWithPrefix(),
					systemContracts.FlowStorageFees.Address.HexWithPrefix(),
					systemContracts.ClusterQC.Address.HexWithPrefix(),
					systemContracts.DKG.Address.HexWithPrefix(),
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
					systemContracts.FungibleToken.Address.HexWithPrefix(),
					systemContracts.FlowToken.Address.HexWithPrefix(),
					systemContracts.IDTableStaking.Address.HexWithPrefix(),
					stakingProxyAddress.HexWithPrefix(),
					systemContracts.FlowStorageFees.Address.HexWithPrefix(),
				),
			},
		},

		// token related contracts
		NewSystemContractChange(
			systemContracts.FlowFees,
			coreContracts.FlowFees(
				systemContracts.FungibleToken.Address.HexWithPrefix(),
				systemContracts.FlowToken.Address.HexWithPrefix(),
				systemContracts.FlowStorageFees.Address.HexWithPrefix(),
			),
		),
		NewSystemContractChange(
			systemContracts.FlowToken,
			coreContracts.FlowToken(
				systemContracts.FungibleToken.Address.HexWithPrefix(),
				fungibleTokenMetadataViewsAddress.HexWithPrefix(),
				systemContracts.MetadataViews.Address.HexWithPrefix(),
				systemContracts.ViewResolver.Address.HexWithPrefix(),
			),
		),
		NewSystemContractChange(
			systemContracts.FungibleToken,
			ftContracts.FungibleToken(
				// Use `Hex()`, since this method adds the prefix.
				systemContracts.ViewResolver.Address.Hex(),
				systemContracts.FlowServiceAccount.Address.Hex(),
			),
		),
		{
			Address: fungibleTokenMetadataViewsAddress,
			Contract: Contract{
				Name: "FungibleTokenMetadataViews",
				Code: ftContracts.FungibleTokenMetadataViews(
					// Use `Hex()`, since this method adds the prefix.
					systemContracts.FungibleToken.Address.Hex(),
					systemContracts.MetadataViews.Address.Hex(),
					systemContracts.ViewResolver.Address.Hex(),
				),
			},
		},

		// NFT related contracts
		NewSystemContractChange(
			systemContracts.NonFungibleToken,
			nftContracts.NonFungibleToken(
				sdk.Address(systemContracts.ViewResolver.Address),
			),
		),
		NewSystemContractChange(
			systemContracts.MetadataViews,
			nftContracts.MetadataViews(
				sdk.Address(systemContracts.FungibleToken.Address),
				sdk.Address(systemContracts.NonFungibleToken.Address),
				sdk.Address(systemContracts.ViewResolver.Address),
			),
		),
		NewSystemContractChange(
			systemContracts.ViewResolver,
			nftContracts.ViewResolver(),
		),
	}

	if chainID != flow.Emulator {
		contractChanges = append(
			contractChanges,
			StagedContract{
				Address: fungibleTokenSwitchboardAddress,
				Contract: Contract{
					Name: "FungibleTokenSwitchboard",
					Code: ftContracts.FungibleTokenSwitchboard(
						systemContracts.FungibleToken.Address.HexWithPrefix(),
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
					systemContracts.FlowToken.Address,
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

func NewSystemContactsMigration(
	chainID flow.ChainID,
	options SystemContractChangesOptions,
) *ChangeContractCodeMigration {
	migration := NewChangeContractCodeMigration(chainID)
	for _, change := range SystemContractChanges(chainID, options) {
		migration.RegisterContractChange(change)
	}
	return migration
}
