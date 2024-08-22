package migrations

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/pretty"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/model/flow"
)

const contractCheckingReporterName = "contract-checking"
const contractCountEstimate = 1000

type AddressContract struct {
	location common.AddressLocation
	code     []byte
}

// NewContractCheckingMigration returns a migration that checks all contracts.
// It parses and checks all contract code and stores the programs in the provided map.
// Important locations is a set of locations that should always succeed to check.
func NewContractCheckingMigration(
	log zerolog.Logger,
	rwf reporters.ReportWriterFactory,
	chainID flow.ChainID,
	verboseErrorOutput bool,
	importantLocations map[common.AddressLocation]struct{},
	programs map[common.Location]*interpreter.Program,
) RegistersMigration {
	return func(registersByAccount *registers.ByAccount) error {

		reporter := rwf.ReportWriter(contractCheckingReporterName)
		defer reporter.Close()

		mr, err := NewInterpreterMigrationRuntime(
			registersByAccount,
			chainID,
			InterpreterMigrationRuntimeConfig{},
		)
		if err != nil {
			return fmt.Errorf("failed to create interpreter migration runtime: %w", err)
		}

		contracts, err := gatherContractsFromRegisters(registersByAccount, log)
		if err != nil {
			return err
		}

		contractsForPrettyPrinting := make(map[common.Location][]byte, len(contracts))
		for _, contract := range contracts {
			contractsForPrettyPrinting[contract.location] = contract.code
		}

		// Check all contracts

		for _, contract := range contracts {
			checkContract(
				contract,
				log,
				mr,
				contractsForPrettyPrinting,
				verboseErrorOutput,
				reporter,
				importantLocations,
				programs,
			)
		}

		return nil
	}
}

func gatherContractsFromRegisters(registersByAccount *registers.ByAccount, log zerolog.Logger) ([]AddressContract, error) {
	log.Info().Msg("Gathering contracts ...")

	contracts := make([]AddressContract, 0, contractCountEstimate)

	err := registersByAccount.ForEachAccount(func(accountRegisters *registers.AccountRegisters) error {
		owner := accountRegisters.Owner()

		encodedContractNames, err := accountRegisters.Get(owner, flow.ContractNamesKey)
		if err != nil {
			return err
		}

		contractNames, err := environment.DecodeContractNames(encodedContractNames)
		if err != nil {
			return err
		}

		for _, contractName := range contractNames {

			contractKey := flow.ContractKey(contractName)

			code, err := accountRegisters.Get(owner, contractKey)
			if err != nil {
				return err
			}

			if len(bytes.TrimSpace(code)) == 0 {
				continue
			}

			address := common.Address([]byte(owner))
			location := common.AddressLocation{
				Address: address,
				Name:    contractName,
			}

			contracts = append(
				contracts,
				AddressContract{
					location: location,
					code:     code,
				},
			)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get contracts of accounts: %w", err)
	}

	sort.Slice(contracts, func(i, j int) bool {
		a := contracts[i]
		b := contracts[j]
		return a.location.ID() < b.location.ID()
	})

	log.Info().Msgf("Gathered all contracts (%d)", len(contracts))
	return contracts, nil
}

func checkContract(
	contract AddressContract,
	log zerolog.Logger,
	mr *InterpreterMigrationRuntime,
	contractsForPrettyPrinting map[common.Location][]byte,
	verboseErrorOutput bool,
	reporter reporters.ReportWriter,
	importantLocations map[common.AddressLocation]struct{},
	programs map[common.Location]*interpreter.Program,
) {
	location := contract.location
	code := contract.code

	log.Info().Msgf("checking contract %s ...", location)

	// Check contract code
	const getAndSetProgram = true
	program, err := mr.ContractAdditionHandler.ParseAndCheckProgram(code, location, getAndSetProgram)
	if err != nil {

		// Pretty print the error
		var builder strings.Builder
		errorPrinter := pretty.NewErrorPrettyPrinter(&builder, false)

		printErr := errorPrinter.PrettyPrintError(err, location, contractsForPrettyPrinting)

		var errorDetails string
		if printErr == nil {
			errorDetails = builder.String()
		} else {
			errorDetails = err.Error()
		}

		if _, ok := importantLocations[location]; ok {
			log.Error().Msgf(
				"error checking important contract %s: %s",
				location,
				errorDetails,
			)
		} else if verboseErrorOutput {
			log.Error().Msgf(
				"error checking contract %s: %s",
				location,
				errorDetails,
			)
		}

		reporter.Write(contractCheckingFailure{
			AccountAddress: location.Address,
			ContractName:   location.Name,
			Code:           string(code),
			Error:          errorDetails,
		})

		return
	}

	// Record the checked program for future use
	programs[location] = program

	reporter.Write(contractCheckingSuccess{
		AccountAddress: location.Address,
		ContractName:   location.Name,
		Code:           string(code),
	})

	log.Info().Msgf("finished checking contract %s", location)
}

type contractCheckingFailure struct {
	AccountAddress common.Address
	ContractName   string
	Code           string
	Error          string
}

var _ json.Marshaler = contractCheckingFailure{}

func (e contractCheckingFailure) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind           string `json:"kind"`
		AccountAddress string `json:"address"`
		ContractName   string `json:"name"`
		Code           string `json:"code"`
		Error          string `json:"error"`
	}{
		Kind:           "checking-failure",
		AccountAddress: e.AccountAddress.HexWithPrefix(),
		ContractName:   e.ContractName,
		Code:           e.Code,
		Error:          e.Error,
	})
}

type contractCheckingSuccess struct {
	AccountAddress common.Address
	ContractName   string
	Code           string
}

var _ json.Marshaler = contractCheckingSuccess{}

func (e contractCheckingSuccess) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind           string `json:"kind"`
		AccountAddress string `json:"address"`
		ContractName   string `json:"name"`
		Code           string `json:"code"`
	}{
		Kind:           "checking-success",
		AccountAddress: e.AccountAddress.HexWithPrefix(),
		ContractName:   e.ContractName,
		Code:           e.Code,
	})
}
