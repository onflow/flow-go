package migrations

import (
	"context"
	"fmt"
	"sync"

	"github.com/rs/zerolog"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/old_parser"
	"github.com/onflow/cadence/runtime/parser"
	"github.com/onflow/cadence/runtime/stdlib"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
)

type StagedContractsMigration struct {
	log                   zerolog.Logger
	mutex                 sync.RWMutex
	contracts             map[common.Address]map[flow.RegisterID]Contract
	stagedContractsGetter func() []StagedContract
}

type StagedContract struct {
	Contract
	address common.Address
}

type Contract struct {
	name string
	code string
}

var _ AccountBasedMigration = (*StagedContractsMigration)(nil)

func NewStagedContractsMigration(stagedContractsGetter func() []StagedContract) *StagedContractsMigration {
	return &StagedContractsMigration{
		stagedContractsGetter: stagedContractsGetter,
	}
}

func (m *StagedContractsMigration) Close() error {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if len(m.contracts) > 0 {
		return fmt.Errorf("failed to find all contract registers that need to be changed")
	}

	return nil
}

func (m *StagedContractsMigration) InitMigration(
	log zerolog.Logger,
	_ []*ledger.Payload,
	_ int,
) error {
	m.log = log.
		With().
		Str("migration", "StagedContractsMigration").
		Logger()

	m.registerContractUpdates()

	return nil
}

// registerContractUpdates prepares the contract updates as a map for easy lookup.
func (m *StagedContractsMigration) registerContractUpdates() {
	contractChanges := m.stagedContractsGetter()
	for _, contractChange := range contractChanges {
		m.registerContractChange(contractChange)
	}
}

func (m *StagedContractsMigration) registerContractChange(change StagedContract) {
	if m.contracts == nil {
		m.contracts = map[common.Address]map[flow.RegisterID]Contract{}
	}

	address := change.address
	if _, ok := m.contracts[address]; !ok {
		m.contracts[address] = map[flow.RegisterID]Contract{}
	}

	registerID := flow.ContractRegisterID(flow.ConvertAddress(address), change.name)

	_, exist := m.contracts[address][registerID]
	if exist {
		// Staged multiple updates for the same contract.
		// Overwrite the previous update.
		m.log.Warn().Msgf(
			"existing staged update found for contract %s.%s. Previous update will be overwritten.",
			address.HexWithPrefix(),
			change.name,
		)
	}

	m.contracts[address][registerID] = change.Contract
}

func (m *StagedContractsMigration) contractUpdatesForAccount(
	address common.Address,
) (map[flow.RegisterID]Contract, bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	contracts, ok := m.contracts[address]

	// remove address from set of addresses
	// to keep track of which addresses are left to change
	delete(m.contracts, address)

	return contracts, ok
}

func (m *StagedContractsMigration) MigrateAccount(
	_ context.Context,
	address common.Address,
	payloads []*ledger.Payload,
) ([]*ledger.Payload, error) {

	contractUpdates, ok := m.contractUpdatesForAccount(address)
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

		updatedContract, ok := contractUpdates[registerID]
		if !ok {
			// not a contract register, or
			// not interested in this contract
			continue
		}

		name := updatedContract.name
		newCode := updatedContract.code
		oldCode := payload.Value()

		err = m.checkUpdateValidity(name, newCode, oldCode)
		if err != nil {
			m.log.Error().Err(err).
				Msgf(
					"fail to update contract %s in account %s",
					name,
					address.HexWithPrefix(),
				)
		} else {
			// change contract code
			payloads[payloadIndex] = ledger.NewPayload(
				key,
				[]byte(newCode),
			)
		}

		// remove contract from list of contracts to change
		// to keep track of which contracts are left to change
		delete(contractUpdates, registerID)
	}

	if len(contractUpdates) > 0 {
		return nil, fmt.Errorf("failed to find all contract registers that need to be changed")
	}

	return payloads, nil
}

func (m *StagedContractsMigration) checkUpdateValidity(contractName, newCode string, oldCode ledger.Value) error {
	newProgram, err := parser.ParseProgram(nil, []byte(newCode), parser.Config{})
	if err != nil {
		return err
	}

	oldProgram, err := old_parser.ParseProgram(nil, oldCode, old_parser.Config{})
	if err != nil {
		return err
	}

	validator := stdlib.NewLegacyContractUpdateValidator(
		nil,
		contractName,
		oldProgram,
		newProgram,
	)

	return validator.Validate()
}

func GetStagedContracts() []StagedContract {
	// TODO:
	return nil
}
