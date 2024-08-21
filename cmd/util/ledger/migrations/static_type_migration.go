package migrations

import (
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
)

type StaticTypeMigrationRules map[common.TypeID]interpreter.StaticType

// NewStaticTypeMigration returns a type converter function.
// Accepts a `rulesGetter` which return
// This is because this constructor is called at the time of constructing the migrations (e.g: Cadence value migration),
// but the rules can only be finalized after running previous TypeRequirementsExtractingMigration migration.
// i.e: the LegacyTypeRequirements list used by NewCompositeTypeConversionRules is lazily populated.
// So we need to delay the construction of the rules, until after the execution of previous migration.
func NewStaticTypeMigration[T interpreter.StaticType](
	rulesGetter func() StaticTypeMigrationRules,
) func(staticType T) interpreter.StaticType {

	var rules StaticTypeMigrationRules

	return func(original T) interpreter.StaticType {
		// Initialize only once
		if rules == nil {
			rules = rulesGetter()
		}

		// Returning `nil` form the callback indicates the type wasn't converted.
		if rules == nil {
			return nil
		}

		if replacement, ok := rules[original.ID()]; ok {
			return replacement
		}
		return nil
	}
}
