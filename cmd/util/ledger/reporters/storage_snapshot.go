package reporters

import (
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

// NewStorageSnapshotFromPayload returns an instance of StorageSnapshot with
// entries loaded from payloads (should only be used for migration)
func NewStorageSnapshotFromPayload(
	payloads []ledger.Payload,
) snapshot.MapStorageSnapshot {
	snapshot := make(snapshot.MapStorageSnapshot, len(payloads))
	for _, entry := range payloads {
		key, err := entry.Key()
		if err != nil {
			panic(err)
		}

		id := flow.NewRegisterID(
			flow.BytesToAddress(key.KeyParts[0].Value),
			string(key.KeyParts[1].Value))

		snapshot[id] = entry.Value()
	}

	return snapshot
}
