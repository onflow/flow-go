package operation

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

func RetrieveJobLatestIndex(r storage.Reader, queue string, index *uint64) error {
	return RetrieveByKey(r, MakePrefix(codeJobQueuePointer, queue), index)
}

func SetJobLatestIndex(w storage.Writer, queue string, index uint64) error {
	return UpsertByKey(w, MakePrefix(codeJobQueuePointer, queue), index)
}

// RetrieveJobAtIndex returns the entity at the given index
func RetrieveJobAtIndex(r storage.Reader, queue string, index uint64, entity *flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codeJobQueue, queue, index), entity)
}

// InsertJobAtIndex insert an entity ID at the given index
func InsertJobAtIndex(w storage.Writer, queue string, index uint64, entity flow.Identifier) error {
	return UpsertByKey(w, MakePrefix(codeJobQueue, queue, index), entity)
}
