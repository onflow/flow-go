package migrations

import (
	"fmt"
	"strings"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/pretty"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/model/flow"
)

const contractCheckingReporterName = "contract-checking"
const contractCountEstimate = 1000

// NewContractCheckingMigration returns a migration that checks all contracts.
// It parses and checks all contract code and stores the programs in the provided map.
func NewContractCheckingMigration(
	log zerolog.Logger,
	rwf reporters.ReportWriterFactory,
	chainID flow.ChainID,
	verboseErrorOutput bool,
	programs map[common.Location]*interpreter.Program,
) RegistersMigration {
	return func(registersByAccount *registers.ByAccount) error {

		reporter := rwf.ReportWriter(contractCheckingReporterName)

		mr, err := NewInterpreterMigrationRuntime(
			registersByAccount,
			chainID,
			InterpreterMigrationRuntimeConfig{},
		)
		if err != nil {
			return fmt.Errorf("failed to create interpreter migration runtime: %w", err)
		}

		// Gather all contracts

		contractsByLocation := make(map[common.Location][]byte, contractCountEstimate)

		err = registersByAccount.ForEach(func(owner string, key string, value []byte) error {

			// Skip payloads that are not contract code
			contractName := flow.KeyContractName(key)
			if contractName == "" {
				return nil
			}

			address := common.Address([]byte(owner))
			code := value
			location := common.AddressLocation{
				Address: address,
				Name:    contractName,
			}

			contractsByLocation[location] = code

			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to iterate over registers: %w", err)
		}

		// Check all contracts

		for location, code := range contractsByLocation {
			log.Info().Msgf("checking contract %s ...", location)

			// Check contract code
			const getAndSetProgram = true
			program, err := mr.ContractAdditionHandler.ParseAndCheckProgram(code, location, getAndSetProgram)
			if err != nil {

				// Pretty print the error
				var builder strings.Builder
				errorPrinter := pretty.NewErrorPrettyPrinter(&builder, false)

				printErr := errorPrinter.PrettyPrintError(err, location, contractsByLocation)

				var errorDetails string
				if printErr == nil {
					errorDetails = builder.String()
				} else {
					errorDetails = err.Error()
				}

				addressLocation := location.(common.AddressLocation)

				if verboseErrorOutput {
					log.Error().Msgf(
						"error checking contract %s: %s",
						location,
						errorDetails,
					)
				}

				reporter.Write(contractCheckingFailure{
					AccountAddressHex: addressLocation.Address.HexWithPrefix(),
					ContractName:      addressLocation.Name,
					Error:             errorDetails,
				})

				continue
			} else {
				// Record the checked program for future use
				programs[location] = program
			}
		}

		reporter.Close()

		return nil
	}
}

type contractCheckingFailure struct {
	AccountAddressHex string `json:"address"`
	ContractName      string `json:"name"`
	Error             string `json:"error"`
}
