package migrations

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/onflow/cadence/runtime/sema"
	"github.com/rs/zerolog"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/old_parser"
	"github.com/onflow/cadence/runtime/stdlib"

	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
)

type StagedContractsMigration struct {
	log                 zerolog.Logger
	mutex               sync.RWMutex
	contracts           map[common.Address]map[flow.RegisterID]Contract
	contractsByLocation map[common.Location][]byte
	stagedContracts     []StagedContract
}

type StagedContract struct {
	Contract
	Address common.Address
}

type Contract struct {
	Name string
	Code []byte
}

var _ AccountBasedMigration = &StagedContractsMigration{}

func NewStagedContractsMigration(stagedContracts []StagedContract) *StagedContractsMigration {
	return &StagedContractsMigration{
		stagedContracts:     stagedContracts,
		contracts:           map[common.Address]map[flow.RegisterID]Contract{},
		contractsByLocation: map[common.Location][]byte{},
	}
}

func (m *StagedContractsMigration) Close() error {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if len(m.contracts) > 0 {
		var sb strings.Builder
		sb.WriteString("failed to find all contract registers that need to be changed:\n")
		for address, contracts := range m.contracts {
			_, _ = fmt.Fprintf(&sb, "- address: %s\n", address)
			for registerID := range contracts {
				_, _ = fmt.Fprintf(&sb, "  - %s\n", flow.RegisterIDContractName(registerID))
			}
		}
		return fmt.Errorf(sb.String())

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
	for _, contractChange := range m.stagedContracts {
		m.registerContractChange(contractChange)
	}
}

func (m *StagedContractsMigration) registerContractChange(change StagedContract) {
	address := change.Address
	if _, ok := m.contracts[address]; !ok {
		m.contracts[address] = map[flow.RegisterID]Contract{}
	}

	registerID := flow.ContractRegisterID(flow.ConvertAddress(address), change.Name)

	_, exist := m.contracts[address][registerID]
	if exist {
		// Staged multiple updates for the same contract.
		// Overwrite the previous update.
		m.log.Warn().Msgf(
			"existing staged update found for contract %s.%s. Previous update will be overwritten.",
			address.HexWithPrefix(),
			change.Name,
		)
	}

	m.contracts[address][registerID] = change.Contract

	location := common.AddressLocation{
		Name:    change.Name,
		Address: address,
	}
	m.contractsByLocation[location] = change.Contract.Code
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

	config := util.RuntimeInterfaceConfig{
		GetContractCodeFunc: func(location runtime.Location) ([]byte, error) {
			// TODO: also consider updated system contracts
			return m.contractsByLocation[location], nil
		},
	}

	mr, err := newMigratorRuntime(address, payloads, config)
	if err != nil {
		return nil, err
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

		name := updatedContract.Name
		newCode := updatedContract.Code
		oldCode := payload.Value()

		err = m.checkUpdateValidity(mr, address, name, newCode, oldCode)
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
				newCode,
			)
		}

		// remove contract from list of contracts to change
		// to keep track of which contracts are left to change
		delete(contractUpdates, registerID)
	}

	if len(contractUpdates) > 0 {
		var sb strings.Builder
		_, _ = fmt.Fprintf(&sb, "failed to find all contract registers that need to be changed for address %s:\n", address)
		for registerID := range contractUpdates {
			_, _ = fmt.Fprintf(&sb, "  - %s\n", flow.RegisterIDContractName(registerID))
		}
		return nil, fmt.Errorf(sb.String())
	}

	return payloads, nil
}

func (m *StagedContractsMigration) checkUpdateValidity(
	mr *migratorRuntime,
	address common.Address,
	contractName string,
	newCode []byte,
	oldCode ledger.Value,
) error {
	location := common.AddressLocation{
		Name:    contractName,
		Address: address,
	}

	// NOTE: do NOT use the program obtained from the host environment, as the current program.
	// Always re-parse and re-check the new program.
	// NOTE: *DO NOT* store the program – the new or updated program
	// should not be effective during the execution
	const getAndSetProgram = false

	newProgram, err := mr.ContractAdditionHandler.ParseAndCheckProgram(newCode, location, getAndSetProgram)
	if err != nil {
		return err
	}

	oldProgram, err := old_parser.ParseProgram(nil, oldCode, old_parser.Config{})
	if err != nil {
		return err
	}

	validator := stdlib.NewCadenceV042ToV1ContractUpdateValidator(
		location,
		contractName,
		mr.ContractNamesProvider,
		oldProgram,
		newProgram.Program,
		// TODO:
		map[common.Location]*sema.Elaboration{},
	)

	return validator.Validate()
}

func StagedContractsFromCSV(path string) ([]StagedContract, error) {
	if path == "" {
		return nil, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	reader := csv.NewReader(file)

	// Expect 3 fields: address, name, code
	reader.FieldsPerRecord = 3

	var contracts []StagedContract

	for {
		rec, err := reader.Read()
		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, err
		}

		addressHex := rec[0]
		name := rec[1]
		code := rec[2]

		address, err := common.HexToAddress(addressHex)
		if err != nil {
			return nil, err
		}

		contracts = append(contracts, StagedContract{
			Contract: Contract{
				Name: name,
				Code: []byte(code),
			},
			Address: address,
		})
	}

	return contracts, nil
}
