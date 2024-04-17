package migrations

import (
	"fmt"
	"strings"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/pretty"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
)

const contractCheckingReporterName = "contract-checking"

// NewContractCheckingMigration returns a migration that checks all contracts.
// It parses and checks all contract code and stores the programs in the provided map.
func NewContractCheckingMigration(
	log zerolog.Logger,
	rwf reporters.ReportWriterFactory,
	chainID flow.ChainID,
	verboseErrorOutput bool,
	programs map[common.Location]*interpreter.Program,
) ledger.Migration {
	return func(payloads []*ledger.Payload) ([]*ledger.Payload, error) {

		reporter := rwf.ReportWriter(contractCheckingReporterName)

		// Extract payloads that have contract code or contract names,
		// we don't need a payload snapshot with all payloads,
		// because all we do is parse and check all contracts.

		contractPayloads, err := extractContractPayloads(payloads, log)
		if err != nil {
			return nil, fmt.Errorf("failed to extract contract payloads: %w", err)
		}

		mr, err := NewMigratorRuntime(
			contractPayloads,
			chainID,
			MigratorRuntimeConfig{},
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create migrator runtime: %w", err)
		}

		// Gather all contracts

		contractsByLocation := make(map[common.Location][]byte, 1000)

		for _, payload := range contractPayloads {
			registerID, registerValue, err := convert.PayloadToRegister(payload)
			if err != nil {
				return nil, fmt.Errorf("failed to convert payload to register: %w", err)
			}

			// Skip payloads that are not contract code
			contractName := flow.RegisterIDContractName(registerID)
			if contractName == "" {
				continue
			}

			owner := common.Address([]byte(registerID.Owner))
			code := registerValue
			location := common.AddressLocation{
				Address: owner,
				Name:    contractName,
			}

			contractsByLocation[location] = code
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

		// Return the payloads as-is
		return payloads, nil
	}
}

// extractContractPayloads extracts payloads that contain contract code or contract names
func extractContractPayloads(payloads []*ledger.Payload, log zerolog.Logger) (
	contractPayloads []*ledger.Payload,
	err error,
) {
	log.Info().Msg("extracting contract payloads ...")

	var contractCount int

	for _, payload := range payloads {
		registerID, _, err := convert.PayloadToRegister(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to convert payload to register: %w", err)
		}

		// Include payloads which contain contract code
		if flow.IsContractCodeRegisterID(registerID) {
			contractPayloads = append(contractPayloads, payload)

			contractCount++
		}

		// Include payloads which contain contract names
		if flow.IsContractNamesRegisterID(registerID) {
			contractPayloads = append(contractPayloads, payload)
		}
	}

	log.Info().Msgf("extracted %d contracts from payloads", contractCount)

	return contractPayloads, nil
}

type contractCheckingFailure struct {
	AccountAddressHex string `json:"address"`
	ContractName      string `json:"name"`
	Error             string `json:"error"`
}
