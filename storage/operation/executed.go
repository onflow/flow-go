package operation

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

func UpdateExecutedBlock(w storage.Writer, blockID flow.Identifier) error {
	return UpsertByKey(w, MakePrefix(codeExecutedBlock), blockID)
}

func RetrieveExecutedBlock(r storage.Reader, blockID *flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codeExecutedBlock), blockID)
}
