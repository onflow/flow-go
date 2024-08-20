package migrations

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/rs/zerolog"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/old_parser"

	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/model/flow"
)

const typeRequirementExtractingReporterName = "type-requirements-extracting"

type LegacyTypeRequirements struct {
	typeRequirements []TypeRequirement
}

type TypeRequirement struct {
	Address      common.Address
	ContractName string
	TypeName     string
}

func NewTypeRequirementsExtractingMigration(
	log zerolog.Logger,
	rwf reporters.ReportWriterFactory,
	importantLocations map[common.AddressLocation]struct{},
	legacyTypeRequirements *LegacyTypeRequirements,
) RegistersMigration {
	return func(registersByAccount *registers.ByAccount) error {

		reporter := rwf.ReportWriter(typeRequirementExtractingReporterName)
		defer reporter.Close()

		// Gather all contracts

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

				if _, isSystemContract := importantLocations[location]; isSystemContract {
					// System contracts have their own type-changing rules.
					// So do not add them here.
					continue
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
			extractTypeRequirements(
				contract,
				log,
				reporter,
				legacyTypeRequirements,
			)
		}

		return nil
	}
}

func extractTypeRequirements(
	contract AddressContract,
	log zerolog.Logger,
	reporter reporters.ReportWriter,
	legacyTypeRequirements *LegacyTypeRequirements,
) {

	// must be parsed with the old parser.
	program, err := old_parser.ParseProgram(
		nil,
		contract.code,
		old_parser.Config{},
	)

	if err != nil {
		// If the old contract cannot be parsed, then ignore
		return
	}

	contractInterface := program.SoleContractInterfaceDeclaration()
	if contractInterface == nil {
		// Type requirements can only reside inside contract interfaces.
		// Ignore all other programs.
		return
	}

	for _, composites := range contractInterface.DeclarationMembers().Composites() {
		typeRequirement := TypeRequirement{
			Address:      contract.location.Address,
			ContractName: contractInterface.Identifier.Identifier,
			TypeName:     composites.Identifier.Identifier,
		}

		legacyTypeRequirements.typeRequirements = append(legacyTypeRequirements.typeRequirements, typeRequirement)

		reporter.Write(typeRequirementRemovalEntry{
			Address:      typeRequirement.Address,
			ContractName: typeRequirement.ContractName,
			TypeName:     typeRequirement.TypeName,
		})
	}

	log.Info().Msgf("Collected %d type-requirements", len(legacyTypeRequirements.typeRequirements))
}

// cadenceValueMigrationFailureEntry

type typeRequirementRemovalEntry struct {
	Address      common.Address
	ContractName string
	TypeName     string
}

var _ valueMigrationReportEntry = typeRequirementRemovalEntry{}

func (e typeRequirementRemovalEntry) accountAddress() common.Address {
	return e.Address
}

var _ json.Marshaler = typeRequirementRemovalEntry{}

func (e typeRequirementRemovalEntry) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind           string `json:"kind"`
		AccountAddress string `json:"account_address"`
		ContractName   string `json:"contract_name"`
		TypeName       string `json:"type_name"`
	}{
		Kind:           "cadence-type-requirement-remove",
		AccountAddress: e.Address.HexWithPrefix(),
		ContractName:   e.ContractName,
		TypeName:       e.TypeName,
	})
}
