package migrations

import (
	"context"
	"encoding/csv"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/old_parser"
	"github.com/onflow/cadence/runtime/pretty"
	"github.com/onflow/cadence/runtime/sema"
	"github.com/onflow/cadence/runtime/stdlib"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
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
	elaborations                   map[common.Location]*sema.Elaboration
	contractAdditionHandler        stdlib.AccountContractAdditionHandler
	contractNamesProvider          stdlib.AccountContractNamesProvider
	reporter                       reporters.ReportWriter
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

func NewStagedContractsMigration(
	chainID flow.ChainID,
	log zerolog.Logger,
	rwf reporters.ReportWriterFactory,
) *StagedContractsMigration {
	return &StagedContractsMigration{
		name:                "StagedContractsMigration",
		log:                 log,
		chainID:             chainID,
		stagedContracts:     map[common.Address]map[flow.RegisterID]Contract{},
		contractsByLocation: map[common.Location][]byte{},
		reporter:            rwf.ReportWriter("staged-contracts-migrator"),
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

	// Close the report writer so it flushes to file.
	m.reporter.Close()

	if len(m.stagedContracts) > 0 {
		dict := zerolog.Dict()
		for address, contracts := range m.stagedContracts {
			arr := zerolog.Arr()
			for registerID := range contracts {
				arr = arr.Str(flow.RegisterIDContractName(registerID))
			}
			dict = dict.Array(
				address.HexWithPrefix(),
				arr,
			)
		}
		m.log.Error().
			Dict("contracts", dict).
			Msg("failed to find all contract registers that need to be changed")
	}

	return nil
}

func (m *StagedContractsMigration) InitMigration(
	log zerolog.Logger,
	allPayloads []*ledger.Payload,
	_ int,
) error {
	m.log = log.
		With().
		Str("migration", m.name).
		Logger()

	// Manually register burner contract
	burnerLocation := common.AddressLocation{
		Name:    "Burner",
		Address: common.Address(BurnerAddressForChain(m.chainID)),
	}
	m.contractsByLocation[burnerLocation] = coreContracts.Burner()

	// Initialize elaborations, ContractAdditionHandler and ContractNamesProvider.
	// These needs to be initialized using **ALL** payloads, not just the payloads of the account.

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

	// Pass empty address. We are only interested in the created `env` object.
	mr, err := NewMigratorRuntime(common.Address{}, allPayloads, config)
	if err != nil {
		return err
	}

	m.elaborations = elaborations
	m.contractAdditionHandler = mr.ContractAdditionHandler
	m.contractNamesProvider = mr.ContractNamesProvider

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
				address,
				name,
				newCode,
				oldCode,
			)
		}

		if err != nil {
			var builder strings.Builder
			errorPrinter := pretty.NewErrorPrettyPrinter(&builder, false)

			location := common.AddressLocation{
				Name:    name,
				Address: address,
			}
			printErr := errorPrinter.PrettyPrintError(err, location, m.contractsByLocation)

			var errorDetails string
			if printErr == nil {
				errorDetails = builder.String()
			} else {
				errorDetails = err.Error()
			}

			m.log.Error().
				Msgf(
					"failed to update contract %s in account %s: %s",
					name,
					address.HexWithPrefix(),
					errorDetails,
				)

			m.reporter.Write(contractUpdateFailed{
				AccountAddressHex: address.HexWithPrefix(),
				ContractName:      name,
				Error:             errorDetails,
			})
		} else {
			// change contract code
			oldPayloads[payloadIndex] = ledger.NewPayload(
				key,
				newCode,
			)

			m.reporter.Write(contractUpdateSuccessful{
				AccountAddressHex: address.HexWithPrefix(),
				ContractName:      name,
			})
		}

		// remove contract from list of contracts to change
		// to keep track of which contracts are left to change
		delete(contractUpdates, registerID)
	}

	if len(contractUpdates) > 0 {
		arr := zerolog.Arr()
		for registerID := range contractUpdates {
			arr = arr.Str(flow.RegisterIDContractName(registerID))
		}
		m.log.Error().
			Array("contracts", arr).
			Str("address", address.HexWithPrefix()).
			Msg("failed to find all contract registers that need to be changed for address")
	}

	return oldPayloads, nil
}

func (m *StagedContractsMigration) checkContractUpdateValidity(
	address common.Address,
	contractName string,
	newCode []byte,
	oldCode ledger.Value,
) error {
	// Parsing and checking of programs has to be done synchronously.
	m.mutex.Lock()
	defer m.mutex.Unlock()

	location := common.AddressLocation{
		Name:    contractName,
		Address: address,
	}

	// NOTE: do NOT use the program obtained from the host environment, as the current program.
	// Always re-parse and re-check the new program.
	// NOTE: *DO NOT* store the program – the new or updated program
	// should not be effective during the execution
	const getAndSetProgram = false

	newProgram, err := m.contractAdditionHandler.ParseAndCheckProgram(newCode, location, getAndSetProgram)
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
		m.contractNamesProvider,
		oldProgram,
		newProgram,
		m.elaborations,
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

type contractUpdateSuccessful struct {
	AccountAddressHex string `json:"address"`
	ContractName      string `json:"name"`
}

type contractUpdateFailed struct {
	AccountAddressHex string `json:"address"`
	ContractName      string `json:"name"`
	Error             string `json:"error"`
}
