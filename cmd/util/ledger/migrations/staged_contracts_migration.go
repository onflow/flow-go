package migrations

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/rs/zerolog"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/old_parser"
	"github.com/onflow/cadence/runtime/sema"
	"github.com/onflow/cadence/runtime/stdlib"

	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"

	coreContracts "github.com/onflow/flow-core-contracts/lib/go/contracts"
)

type StagedContractsMigration struct {
	name                           string
	chainID                        flow.ChainID
	log                            zerolog.Logger
	mutex                          sync.RWMutex
	stagedContracts                map[common.Address]map[flow.RegisterID]Contract
	contractsByLocation            map[common.Location][]byte
	enableUpdateValidation         bool
	userDefinedTypeChangeCheckFunc func(oldTypeID common.TypeID, newTypeID common.TypeID) (checked bool, valid bool)
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

func NewStagedContractsMigration(chainID flow.ChainID, log zerolog.Logger) *StagedContractsMigration {
	return &StagedContractsMigration{
		name:                "StagedContractsMigration",
		log:                 log,
		chainID:             chainID,
		stagedContracts:     map[common.Address]map[flow.RegisterID]Contract{},
		contractsByLocation: map[common.Location][]byte{},
	}
}

func (m *StagedContractsMigration) WithContractUpdateValidation() *StagedContractsMigration {
	m.enableUpdateValidation = true
	m.userDefinedTypeChangeCheckFunc = newUserDefinedTypeChangeCheckerFunc(m.chainID)
	return m
}

func (m *StagedContractsMigration) WithName(name string) *StagedContractsMigration {
	m.name = name
	return m
}

func (m *StagedContractsMigration) Close() error {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if len(m.stagedContracts) > 0 {
		var sb strings.Builder
		sb.WriteString("failed to find all contract registers that need to be changed:\n")
		for address, contracts := range m.stagedContracts {
			_, _ = fmt.Fprintf(&sb, "- address: %s\n", address.HexWithPrefix())
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
		Str("migration", m.name).
		Logger()

	// Manually register burner contract
	burnerLocation := common.AddressLocation{
		Name:    "Burner",
		Address: common.Address(m.chainID.Chain().ServiceAddress()),
	}
	m.contractsByLocation[burnerLocation] = coreContracts.Burner()

	return nil
}

// RegisterContractUpdates prepares the contract updates as a map for easy lookup.
func (m *StagedContractsMigration) RegisterContractUpdates(stagedContracts []StagedContract) {
	for _, contractChange := range stagedContracts {
		m.RegisterContractChange(contractChange)
	}
}

func (m *StagedContractsMigration) RegisterContractChange(change StagedContract) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	address := change.Address

	chain := m.chainID.Chain()

	if _, err := chain.IndexFromAddress(flow.Address(address)); err != nil {
		m.log.Error().Msgf(
			"invalid contract update: invalid address for chain %s: %s (%s)",
			m.chainID,
			address.HexWithPrefix(),
			change.Name,
		)
	}

	if _, ok := m.stagedContracts[address]; !ok {
		m.stagedContracts[address] = map[flow.RegisterID]Contract{}
	}

	name := change.Name

	registerID := flow.ContractRegisterID(flow.ConvertAddress(address), name)

	_, exist := m.stagedContracts[address][registerID]
	if exist {
		// Staged multiple updates for the same contract.
		// Overwrite the previous update.
		m.log.Warn().Msgf(
			"existing staged update found for contract %s.%s. Previous update will be overwritten.",
			address.HexWithPrefix(),
			name,
		)
	}

	m.stagedContracts[address][registerID] = change.Contract

	location := common.AddressLocation{
		Name:    name,
		Address: address,
	}
	m.contractsByLocation[location] = change.Code
}

func (m *StagedContractsMigration) contractUpdatesForAccount(
	address common.Address,
) (map[flow.RegisterID]Contract, bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	contracts, ok := m.stagedContracts[address]

	// remove address from set of addresses
	// to keep track of which addresses are left to change
	delete(m.stagedContracts, address)

	return contracts, ok
}

func (m *StagedContractsMigration) MigrateAccount(
	_ context.Context,
	address common.Address,
	oldPayloads []*ledger.Payload,
) ([]*ledger.Payload, error) {

	checkPayloadsOwnership(oldPayloads, address, m.log)

	contractUpdates, ok := m.contractUpdatesForAccount(address)
	if !ok {
		// no contracts to change on this address
		return oldPayloads, nil
	}

	elaborations := map[common.Location]*sema.Elaboration{}

	config := util.RuntimeInterfaceConfig{
		GetContractCodeFunc: func(location runtime.Location) ([]byte, error) {
			// TODO: also consider updated system contracts
			return m.contractsByLocation[location], nil
		},
		GetOrLoadProgramListener: func(location runtime.Location, program *interpreter.Program, err error) {
			if err == nil {
				elaborations[location] = program.Elaboration
			}
		},
	}

	mr, err := NewMigratorRuntime(address, oldPayloads, config)
	if err != nil {
		return nil, err
	}

	for payloadIndex, payload := range oldPayloads {
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

		if m.enableUpdateValidation {
			err = m.checkContractUpdateValidity(
				mr,
				address,
				name,
				newCode,
				oldCode,
				elaborations,
			)
		}

		if err != nil {
			m.log.Error().Err(err).
				Msgf(
					"fail to update contract %s in account %s",
					name,
					address.HexWithPrefix(),
				)
		} else {
			// change contract code
			oldPayloads[payloadIndex] = ledger.NewPayload(
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

	return oldPayloads, nil
}

func (m *StagedContractsMigration) checkContractUpdateValidity(
	mr *migratorRuntime,
	address common.Address,
	contractName string,
	newCode []byte,
	oldCode ledger.Value,
	elaborations map[common.Location]*sema.Elaboration,
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
		elaborations,
	)

	validator.WithUserDefinedTypeChangeChecker(
		m.userDefinedTypeChangeCheckFunc,
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

func newUserDefinedTypeChangeCheckerFunc(
	chainID flow.ChainID,
) func(oldTypeID common.TypeID, newTypeID common.TypeID) (checked, valid bool) {

	typeChangeRules := map[common.TypeID]common.TypeID{}

	compositeTypeRules := NewCompositeTypeConversionRules(chainID)
	for typeID, newStaticType := range compositeTypeRules {
		typeChangeRules[typeID] = newStaticType.ID()
	}

	interfaceTypeRules := NewInterfaceTypeConversionRules(chainID)
	for typeID, newStaticType := range interfaceTypeRules {
		typeChangeRules[typeID] = newStaticType.ID()
	}

	return func(oldTypeID common.TypeID, newTypeID common.TypeID) (checked, valid bool) {
		expectedNewTypeID, found := typeChangeRules[oldTypeID]
		if found {
			return true, expectedNewTypeID == newTypeID
		}
		return false, false
	}
}
