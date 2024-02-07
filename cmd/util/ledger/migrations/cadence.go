package migrations

import (
	"bytes"
	_ "embed"

	"github.com/onflow/cadence/migrations/statictypes"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/tools/analysis"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

//go:embed cadence_composite_type_rules.csv
var cadenceCompositeTypeRulesCSV []byte

//go:embed cadence_interface_type_rules.csv
var cadenceInterfaceTypeRulesCSV []byte

var cadenceCompositeTypeConverter statictypes.CompositeTypeConverterFunc
var cadenceInterfaceTypeConverter statictypes.InterfaceTypeConverterFunc

func init() {
	programs := analysis.Programs{}

	cadenceCompositeTypeRules, err := ReadCSVStaticTypeMigrationRules(
		programs,
		bytes.NewReader(cadenceCompositeTypeRulesCSV),
	)
	if err != nil {
		panic(err)
	}

	cadenceCompositeTypeConverter =
		NewStaticTypeMigrator[*interpreter.CompositeStaticType](cadenceCompositeTypeRules)

	cadenceInterfaceTypeRules, err := ReadCSVStaticTypeMigrationRules(
		programs,
		bytes.NewReader(cadenceInterfaceTypeRulesCSV),
	)
	if err != nil {
		panic(err)
	}

	cadenceInterfaceTypeConverter =
		NewStaticTypeMigrator[*interpreter.InterfaceStaticType](cadenceInterfaceTypeRules)
}

func NewCadence1ValueMigrations(
	log zerolog.Logger,
	rwf reporters.ReportWriterFactory,
	nWorker int,
) (
	migrations []ledger.Migration,
) {

	// Populated by CadenceLinkValueMigrator,
	// used by CadenceCapabilityValueMigrator
	capabilityIDs := map[interpreter.AddressPath]interpreter.UInt64Value{}

	for _, accountBasedMigration := range []AccountBasedMigration{
		NewCadence1ValueMigrator(
			rwf,
			cadenceCompositeTypeConverter,
			cadenceInterfaceTypeConverter,
		),
		NewCadence1LinkValueMigrator(rwf, capabilityIDs),
		NewCadence1CapabilityValueMigrator(rwf, capabilityIDs),
	} {
		migrations = append(
			migrations,
			NewAccountBasedMigration(
				log,
				nWorker, []AccountBasedMigration{
					accountBasedMigration,
				},
			),
		)
	}

	return
}

func NewCadence1ContractsMigrations(
	log zerolog.Logger,
	nWorker int,
	chainID flow.ChainID,
	evmContractChange EVMContractChange,
	stagedContracts []StagedContract,
) []ledger.Migration {

	return []ledger.Migration{
		NewAccountBasedMigration(
			log,
			nWorker,
			[]AccountBasedMigration{
				NewSystemContactsMigration(
					chainID,
					SystemContractChangesOptions{
						EVM: evmContractChange,
					},
				),
			},
		),
		NewBurnerDeploymentMigration(chainID, log),
		NewAccountBasedMigration(
			log,
			nWorker,
			[]AccountBasedMigration{
				NewStagedContractsMigration(stagedContracts),
			},
		),
	}
}

func NewCadence1Migrations(
	log zerolog.Logger,
	rwf reporters.ReportWriterFactory,
	nWorker int,
	chainID flow.ChainID,
	evmContractChange EVMContractChange,
	stagedContracts []StagedContract,
) []ledger.Migration {
	return common.Concat(
		NewCadence1ContractsMigrations(log, nWorker, chainID, evmContractChange, stagedContracts),
		NewCadence1ValueMigrations(log, rwf, nWorker),
	)
}
