package migrations

import (
	"fmt"

	"github.com/onflow/cadence/runtime/common"
	coreContracts "github.com/onflow/flow-core-contracts/lib/go/contracts"
	"github.com/rs/zerolog"

	evm "github.com/onflow/flow-go/fvm/evm/stdlib"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
)

type ChangeContractCodeMigration struct {
	*StagedContractsMigration
}

var _ AccountBasedMigration = (*ChangeContractCodeMigration)(nil)

func NewChangeContractCodeMigration(chainID flow.ChainID, log zerolog.Logger) *ChangeContractCodeMigration {
	return &ChangeContractCodeMigration{
		StagedContractsMigration: NewStagedContractsMigration(chainID, log).
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

	env := systemContracts.AsTemplateEnv()
	env.BurnerAddress = BurnerAddressForChain(chainID).Hex()

	switch chainID {
	case flow.Mainnet:
		env.StakingCollectionAddress = "0x8d0e87b65159ae63"
		env.StakingProxyAddress = "0x62430cf28c26d095"

	case flow.Testnet:
		env.StakingCollectionAddress = "0x95e019a17d0e23d7"
		env.StakingProxyAddress = "0x7aad92e5a0715d21"

	case flow.Emulator, flow.Localnet:
		env.StakingCollectionAddress = env.ServiceAccountAddress
		env.StakingProxyAddress = env.ServiceAccountAddress

	default:
		panic(fmt.Errorf("unsupported chain ID: %s", chainID))
	}

	env.LockedTokensAddress = env.StakingCollectionAddress

	contractChanges := []StagedContract{
		// epoch related contracts
		NewSystemContractChange(
			systemContracts.Epoch,
			coreContracts.FlowEpoch(
				env,
			),
		),
		NewSystemContractChange(
			systemContracts.IDTableStaking,
			coreContracts.FlowIDTableStaking(
				env,
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
				env,
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
				env,
			),
		),
		{
			Address: common.Address(flow.HexToAddress(env.StakingCollectionAddress)),
			Contract: Contract{
				Name: "FlowStakingCollection",
				Code: coreContracts.FlowStakingCollection(env),
			},
		},
		{
			Address: common.Address(flow.HexToAddress(env.StakingProxyAddress)),
			Contract: Contract{
				Name: "StakingProxy",
				Code: coreContracts.FlowStakingProxy(),
			},
		},
		{
			Address: common.Address(flow.HexToAddress(env.LockedTokensAddress)),
			Contract: Contract{
				Name: "LockedTokens",
				Code: coreContracts.FlowLockedTokens(env),
			},
		},

		// token related contracts
		NewSystemContractChange(
			systemContracts.FlowFees,
			coreContracts.FlowFees(
				env,
			),
		),
		NewSystemContractChange(
			systemContracts.FlowToken,
			coreContracts.FlowToken(
				env,
			),
		),
		NewSystemContractChange(
			systemContracts.FungibleToken,
			coreContracts.FungibleToken(
				env,
			),
		),
		{
			Address: common.Address(flow.HexToAddress(env.FungibleTokenMetadataViewsAddress)),
			Contract: Contract{
				Name: "FungibleTokenMetadataViews",
				Code: coreContracts.FungibleTokenMetadataViews(env),
			},
		},

		// NFT related contracts
		NewSystemContractChange(
			systemContracts.NonFungibleToken,
			coreContracts.NonFungibleToken(
				env,
			),
		),
		NewSystemContractChange(
			systemContracts.MetadataViews,
			coreContracts.MetadataViews(
				env,
			),
		),
		NewSystemContractChange(
			systemContracts.ViewResolver,
			coreContracts.ViewResolver(),
		),
	}

	switch chainID {
	case flow.Emulator, flow.Localnet:
		// skip

	default:
		contractChanges = append(
			contractChanges,
			StagedContract{
				Address: common.Address(flow.HexToAddress(env.FungibleTokenSwitchboardAddress)),
				Contract: Contract{
					Name: "FungibleTokenSwitchboard",
					Code: coreContracts.FungibleTokenSwitchboard(env),
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
					flow.HexToAddress(env.FlowTokenAddress),
					abiOnly,
				),
			),
		)
	default:
		panic(fmt.Errorf("unsupported EVM contract change option: %d", options.EVM))
	}

	return contractChanges
}

func NewSystemContactsMigration(
	chainID flow.ChainID,
	log zerolog.Logger,
	options SystemContractChangesOptions,
) *ChangeContractCodeMigration {
	migration := NewChangeContractCodeMigration(chainID, log)
	for _, change := range SystemContractChanges(chainID, options) {
		migration.RegisterContractChange(change)
	}
	return migration
}
