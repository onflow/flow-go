package operation

import (
	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/model/flow"

	"github.com/dgraph-io/badger/v2"
)

func InsertExecutionStateInteractions(blockID flow.Identifier, interactions []*delta.Snapshot) func(*badger.Txn) error {
	return insert(makePrefix(codeExecutionStateInteractions, blockID), interactions)
}

func RetrieveExecutionStateInteractions(blockID flow.Identifier, interactions *[]*delta.Snapshot) func(*badger.Txn) error {
	return retrieve(makePrefix(codeExecutionStateInteractions, blockID), interactions)
}

// FindExecutionStateInteractions iterates through all execution state interactions, calling `filter` on each, and adding
// them to the `found` slice if `filter` returned true
// For legacy migration purposes only, needs to reside in this package to use internal functions
// TODO Remove after migration
func FindLegacyExecutionStateInteractions(filter func(interactions []*delta.LegacySnapshot) bool, found *[][]*delta.LegacySnapshot) func(*badger.Txn) error {
	return traverse(makePrefix(codeExecutionStateInteractions), func() (checkFunc, createFunc, handleFunc) {
		check := func(key []byte) bool {
			return true
		}
		var val []*delta.LegacySnapshot
		create := func() interface{} {
			return &val
		}
		handle := func() error {
			if filter(val) {
				*found = append(*found, val)
			}
			return nil
		}
		return check, create, handle
	})
}
