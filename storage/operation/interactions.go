package operation

import (
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

func InsertExecutionStateInteractions(
	w storage.Writer,
	blockID flow.Identifier,
	executionSnapshots []*snapshot.ExecutionSnapshot,
) error {
	return UpsertByKey(w,
		MakePrefix(codeExecutionStateInteractions, blockID),
		executionSnapshots)
}

func RetrieveExecutionStateInteractions(
	r storage.Reader,
	blockID flow.Identifier,
	executionSnapshots *[]*snapshot.ExecutionSnapshot,
) error {
	return RetrieveByKey(r,
		MakePrefix(codeExecutionStateInteractions, blockID), executionSnapshots)
}
