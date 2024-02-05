package migrations

import (
	"bytes"
	_ "embed"

	"github.com/onflow/cadence/migrations/statictypes"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/tools/analysis"

	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
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

func NewCadenceMigrations(
	rwf reporters.ReportWriterFactory,
) []AccountBasedMigration {

	// Populated by CadenceLinkValueMigrator, used by CadenceValueMigrator
	capabilityIDs := map[interpreter.AddressPath]interpreter.UInt64Value{}

	return []AccountBasedMigration{
		NewCadenceLinkValueMigrator(rwf, capabilityIDs),
		NewCadenceValueMigrator(
			rwf,
			capabilityIDs,
			cadenceCompositeTypeConverter,
			cadenceInterfaceTypeConverter,
		),
	}
}
