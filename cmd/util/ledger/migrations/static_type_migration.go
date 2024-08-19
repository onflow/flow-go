package migrations

import (
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
)

type StaticTypeMigrationRules map[common.TypeID]interpreter.StaticType

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
