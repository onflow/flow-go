package operation

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

func InsertEpochSetup(w storage.Writer, eventID flow.Identifier, event *flow.EpochSetup) error {
	return UpsertByKey(w, MakePrefix(codeEpochSetup, eventID), event)
}

func RetrieveEpochSetup(r storage.Reader, eventID flow.Identifier, event *flow.EpochSetup) error {
	return RetrieveByKey(r, MakePrefix(codeEpochSetup, eventID), event)
}

func InsertEpochCommit(w storage.Writer, eventID flow.Identifier, event *flow.EpochCommit) error {
	return UpsertByKey(w, MakePrefix(codeEpochCommit, eventID), event)
}

func RetrieveEpochCommit(r storage.Reader, eventID flow.Identifier, event *flow.EpochCommit) error {
	return RetrieveByKey(r, MakePrefix(codeEpochCommit, eventID), event)
}
