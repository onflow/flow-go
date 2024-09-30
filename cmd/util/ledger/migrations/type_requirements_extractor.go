package migrations

import (
	"encoding/json"

	"github.com/rs/zerolog"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/old_parser"

	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
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

		contracts, err := GatherContractsFromRegisters(registersByAccount, log)
		if err != nil {
			return err
		}

		// Extract type requirements from all contracts

		for _, contract := range contracts {
			if _, isSystemContract := importantLocations[contract.Location]; isSystemContract {
				// System contracts have their own type-changing rules.
				// So do not add them here.
				continue
			}

			ExtractTypeRequirements(
				contract,
				log,
				reporter,
				legacyTypeRequirements,
			)
		}

		return nil
	}
}

func ExtractTypeRequirements(
	contract AddressContract,
	log zerolog.Logger,
	reporter reporters.ReportWriter,
	legacyTypeRequirements *LegacyTypeRequirements,
) {

	// must be parsed with the old parser.
	program, err := old_parser.ParseProgram(
		nil,
		contract.Code,
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
			Address:      contract.Location.Address,
			ContractName: contractInterface.Identifier.Identifier,
			TypeName:     composites.Identifier.Identifier,
		}

		legacyTypeRequirements.typeRequirements = append(legacyTypeRequirements.typeRequirements, typeRequirement)

		reporter.Write(typeRequirementRemovalEntry{
			TypeRequirement: typeRequirement,
		})
	}

	log.Info().Msgf("Collected %d type-requirements", len(legacyTypeRequirements.typeRequirements))
}

// cadenceValueMigrationFailureEntry

type typeRequirementRemovalEntry struct {
	TypeRequirement
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
