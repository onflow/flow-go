package operation

import (
	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/flow"
)

func InsertExecutionStateInteractions(
	blockID flow.Identifier,
	executionSnapshots []*snapshot.ExecutionSnapshot,
) func(pebble.Writer) error {
	return insert(
		makePrefix(codeExecutionStateInteractions, blockID),
		executionSnapshots)
}

func RetrieveExecutionStateInteractions(
	blockID flow.Identifier,
	executionSnapshots *[]*snapshot.ExecutionSnapshot,
) func(pebble.Reader) error {
	return retrieve(
		makePrefix(codeExecutionStateInteractions, blockID), executionSnapshots)
}
