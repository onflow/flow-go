package migrations

import (
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
		defer reporter.Close()

		mr, err := NewInterpreterMigrationRuntime(
			registersByAccount,
			chainID,
			InterpreterMigrationRuntimeConfig{},
		)
		if err != nil {
			return fmt.Errorf("failed to create interpreter migration runtime: %w", err)
		}

		// Gather all contracts

		log.Info().Msg("Gathering contracts ...")

		contractsForPrettyPrinting := make(map[common.Location][]byte, contractCountEstimate)

		type contract struct {
			location common.AddressLocation
			code     []byte
		}
		contracts := make([]contract, 0, contractCountEstimate)

		err = registersByAccount.ForEachAccount(func(accountRegisters *registers.AccountRegisters) error {
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

				address := common.Address([]byte(owner))
				location := common.AddressLocation{
					Address: address,
					Name:    contractName,
				}

				contracts = append(
					contracts,
					contract{
						location: location,
						code:     code,
					},
				)

				contractsForPrettyPrinting[location] = code
			}

			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to get contracts of accounts: %w", err)
		}

		sort.Slice(contracts, func(i, j int) bool {
			a := contracts[i]
			b := contracts[j]
			return a.location.ID() < b.location.ID()
		})

		log.Info().Msgf("Gathered all contracts (%d)", len(contracts))

		// Check all contracts

		for _, contract := range contracts {
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

				if verboseErrorOutput {
					log.Error().Msgf(
						"error checking contract %s: %s",
						location,
						errorDetails,
					)
				}

				reporter.Write(contractCheckingFailure{
					AccountAddressHex: location.Address.HexWithPrefix(),
					ContractName:      location.Name,
					Error:             errorDetails,
				})

				continue
			} else {
				// Record the checked program for future use
				programs[location] = program
			}
		}

		return nil
	}
}

type contractCheckingFailure struct {
	AccountAddressHex string `json:"address"`
	ContractName      string `json:"name"`
	Error             string `json:"error"`
}
