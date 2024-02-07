package migrations

import (
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
)

type StaticTypeMigrationRules map[common.TypeID]interpreter.StaticType

func NewStaticTypeMigrator[T interpreter.StaticType](
	rules StaticTypeMigrationRules,
) func(staticType T) interpreter.StaticType {

	// Returning `nil` form the callback indicates the type wasn't converted.

	if rules == nil {
		return func(original T) interpreter.StaticType {
			return nil
		}
	}

	return func(original T) interpreter.StaticType {
		if replacement, ok := rules[original.ID()]; ok {
			return replacement
		}
		return nil
	}
}
