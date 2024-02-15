package migrations

import (
	"context"
	"fmt"
	"strings"
	"sync"

	coreContracts "github.com/onflow/flow-core-contracts/lib/go/contracts"
	coreContractstemplates "github.com/onflow/flow-core-contracts/lib/go/templates"
	"github.com/rs/zerolog"

	"github.com/onflow/cadence/runtime/common"

	evm "github.com/onflow/flow-go/fvm/evm/stdlib"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
)

type ChangeContractCodeMigration struct {
	log       zerolog.Logger
	mutex     sync.RWMutex
	contracts map[common.Address]map[flow.RegisterID]string
}

var _ AccountBasedMigration = (*ChangeContractCodeMigration)(nil)

func (d *ChangeContractCodeMigration) Close() error {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	if len(d.contracts) > 0 {
		var sb strings.Builder
		sb.WriteString("failed to find all contract registers that need to be changed:\n")
		for address, contracts := range d.contracts {
			_, _ = fmt.Fprintf(&sb, "- address: %s\n", address)
			for registerID := range contracts {
				_, _ = fmt.Fprintf(&sb, "  - %s\n", flow.RegisterIDContractName(registerID))
			}
		}
		return fmt.Errorf(sb.String())
	}

	return nil
}

func (d *ChangeContractCodeMigration) InitMigration(
	log zerolog.Logger,
	_ []*ledger.Payload,
	_ int,
) error {
	d.log = log.
		With().
		Str("migration", "ChangeContractCodeMigration").
		Logger()

	return nil
}

func (d *ChangeContractCodeMigration) MigrateAccount(
	_ context.Context,
	address common.Address,
	payloads []*ledger.Payload,
) ([]*ledger.Payload, error) {

	contracts, ok := (func() (map[flow.RegisterID]string, bool) {
		d.mutex.Lock()
		defer d.mutex.Unlock()

		contracts, ok := d.contracts[address]

		// remove address from set of addresses
		// to keep track of which addresses are left to change
		delete(d.contracts, address)

		return contracts, ok
	})()

	if !ok {
		// no contracts to change on this address
		return payloads, nil
	}

	for payloadIndex, payload := range payloads {
		key, err := payload.Key()
		if err != nil {
			return nil, err
		}

		registerID, err := convert.LedgerKeyToRegisterID(key)
		if err != nil {
			return nil, err
		}

		newContract, ok := contracts[registerID]
		if !ok {
			// not a contract register, or
			// not interested in this contract
			continue
		}

		// change contract code
		payloads[payloadIndex] = ledger.NewPayload(
			key,
			[]byte(newContract),
		)

		// TODO: maybe log diff between old and new

		// remove contract from list of contracts to change
		// to keep track of which contracts are left to change
		delete(contracts, registerID)
	}

	if len(contracts) > 0 {
		var sb strings.Builder
		_, _ = fmt.Fprintf(&sb, "failed to find all contract registers that need to be changed for address %s:\n", address)
		for registerID := range contracts {
			_, _ = fmt.Fprintf(&sb, "- %s\n", flow.RegisterIDContractName(registerID))
		}
		return nil, fmt.Errorf(sb.String())
	}

	return payloads, nil
}

func (d *ChangeContractCodeMigration) RegisterContractChange(
	address common.Address,
	contractName string,
	newContractCode string,
) (
	previousNewContractCode string,
) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if d.contracts == nil {
		d.contracts = map[common.Address]map[flow.RegisterID]string{}
	}

	if _, ok := d.contracts[address]; !ok {
		d.contracts[address] = map[flow.RegisterID]string{}
	}

	registerID := flow.ContractRegisterID(flow.ConvertAddress(address), contractName)

	previousNewContractCode = d.contracts[address][registerID]

	d.contracts[address][registerID] = newContractCode

	return
}

type SystemContractChange struct {
	Address         common.Address
	ContractName    string
	NewContractCode string
}

func NewSystemContractChange(
	systemContract systemcontracts.SystemContract,
	newContractCode []byte,
) SystemContractChange {
	return SystemContractChange{
		Address:         common.Address(systemContract.Address),
		ContractName:    systemContract.Name,
		NewContractCode: string(newContractCode),
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

func SystemContractChanges(chainID flow.ChainID, options SystemContractChangesOptions) []SystemContractChange {
	systemContracts := systemcontracts.SystemContractsForChain(chainID)

	env := coreContractstemplates.Environment{
		ServiceAccountAddress:             systemContracts.FlowServiceAccount.Address.Hex(),
		ViewResolverAddress:               systemContracts.ViewResolver.Address.Hex(),
		BurnerAddress:                     BurnerAddressForChain(chainID).Hex(),
		FungibleTokenAddress:              systemContracts.FungibleToken.Address.Hex(),
		NonFungibleTokenAddress:           systemContracts.NonFungibleToken.Address.Hex(),
		MetadataViewsAddress:              systemContracts.MetadataViews.Address.Hex(),
		FungibleTokenMetadataViewsAddress: systemContracts.FungibleToken.Address.Hex(),
		FungibleTokenSwitchboardAddress:   systemContracts.FungibleToken.Address.Hex(),
		FlowTokenAddress:                  systemContracts.FlowToken.Address.Hex(),
		IDTableAddress:                    systemContracts.IDTableStaking.Address.Hex(),
		QuorumCertificateAddress:          systemContracts.ClusterQC.Address.Hex(),
		DkgAddress:                        systemContracts.DKG.Address.Hex(),
		FlowFeesAddress:                   systemContracts.FlowFees.Address.Hex(),
		StorageFeesAddress:                systemContracts.FlowStorageFees.Address.Hex(),
	}

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

	contractChanges := []SystemContractChange{
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
			Address:      common.Address(flow.HexToAddress(env.StakingCollectionAddress)),
			ContractName: "FlowStakingCollection",
			NewContractCode: string(coreContracts.FlowStakingCollection(
				env,
			)),
		},
		{
			Address:         common.Address(flow.HexToAddress(env.StakingProxyAddress)),
			ContractName:    "StakingProxy",
			NewContractCode: string(coreContracts.FlowStakingProxy()),
		},
		{
			Address:      common.Address(flow.HexToAddress(env.LockedTokensAddress)),
			ContractName: "LockedTokens",
			NewContractCode: string(coreContracts.FlowLockedTokens(
				env,
			)),
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
			Address:      common.Address(flow.HexToAddress(env.FungibleTokenMetadataViewsAddress)),
			ContractName: "FungibleTokenMetadataViews",
			NewContractCode: string(coreContracts.FungibleTokenMetadataViews(
				env,
			)),
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
			SystemContractChange{
				Address:      common.Address(flow.HexToAddress(env.FungibleTokenSwitchboardAddress)),
				ContractName: "FungibleTokenSwitchboard",
				NewContractCode: string(coreContracts.FungibleTokenSwitchboard(
					env,
				)),
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
	migration := &ChangeContractCodeMigration{}
	for _, change := range SystemContractChanges(chainID, options) {
		migration.RegisterContractChange(
			change.Address,
			change.ContractName,
			change.NewContractCode,
		)
	}
	return migration
}
