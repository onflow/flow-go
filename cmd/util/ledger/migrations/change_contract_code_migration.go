package migrations

import (
	"context"
	"fmt"
	"sync"

	coreContracts "github.com/onflow/flow-core-contracts/lib/go/contracts"
	ftContracts "github.com/onflow/flow-ft/lib/go/contracts"
	sdk "github.com/onflow/flow-go-sdk"
	nftContracts "github.com/onflow/flow-nft/lib/go/contracts"
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
		return fmt.Errorf("failed to find all contract registers that need to be changed")
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

	for i, payload := range payloads {
		key, err := payload.Key()
		if err != nil {
			return nil, err
		}
		registerID, err := convert.LedgerKeyToRegisterID(key)
		newContract, ok := contracts[registerID]
		if !ok {
			// not a contract register, or
			// not interested in this contract
			continue
		}

		// change contract code
		payloads[i] = ledger.NewPayload(
			key,
			[]byte(newContract),
		)

		// remove contract from list of contracts to change
		// to keep track of which contracts are left to change
		delete(contracts, registerID)
	}

	if len(contracts) > 0 {
		return nil, fmt.Errorf("failed to find all contract registers that need to be changed")
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

func SystemContractChanges(chainID flow.ChainID) []SystemContractChange {
	systemContracts := systemcontracts.SystemContractsForChain(chainID)

	return []SystemContractChange{
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
				systemContracts.MetadataViews.Address.HexWithPrefix(),
				systemContracts.ViewResolver.Address.HexWithPrefix(),
			),
		),
		NewSystemContractChange(
			systemContracts.FungibleToken,
			ftContracts.FungibleToken(),
		),

		// NFT related contracts
		NewSystemContractChange(
			systemContracts.NonFungibleToken,
			nftContracts.NonFungibleToken(),
		),
		NewSystemContractChange(
			systemContracts.MetadataViews,
			nftContracts.MetadataViews(
				sdk.Address(systemContracts.FungibleToken.Address),
				sdk.Address(systemContracts.NonFungibleToken.Address),
			),
		),
		NewSystemContractChange(
			systemContracts.ViewResolver,
			nftContracts.Resolver(),
		),

		// EVM related contracts
		NewSystemContractChange(
			systemContracts.EVM,
			evm.ContractCode(
				systemContracts.FlowToken.Address,
			),
		),
	}
}
