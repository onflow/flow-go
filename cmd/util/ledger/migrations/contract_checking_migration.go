package migrations

import (
	"fmt"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
)

// NewContractCheckingMigration returns a migration that checks all contracts.
func NewContractCheckingMigration(
	log zerolog.Logger,
	programs map[common.Location]*interpreter.Program,
) ledger.Migration {
	return func(payloads []*ledger.Payload) ([]*ledger.Payload, error) {

		// Extract payloads that have contract code or contract names,
		// we don't need a payload snapshot with all payloads,
		// because all we do is parse and check all contracts.

		contractPayloads, err := extractContractPayloads(payloads, log)
		if err != nil {
			return nil, fmt.Errorf("failed to extract contract payloads: %w", err)
		}

		mr, err := NewMigratorRuntime(
			contractPayloads,
			MigratorRuntimeConfig{},
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create migrator runtime: %w", err)
		}

		// Check all contracts

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

			log.Info().Msgf("checking contract %s ...", location)

			// Check contract code
			const getAndSetProgram = true
			program, err := mr.ContractAdditionHandler.ParseAndCheckProgram(code, location, getAndSetProgram)
			if err != nil {
				// TODO: report, pretty printed. like in StageContractsMigration
				log.Error().Err(err).Msgf("failed to check contract %s", location)
				continue
			}

			programs[location] = program
		}

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
