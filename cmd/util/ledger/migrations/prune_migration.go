package migrations

import (
	"github.com/onflow/flow-go/ledger"
)

// PruneMigration removes all the payloads with empty value
// this prunes the trie for values that have been deleted
func PruneMigration(payload []ledger.Payload) ([]ledger.Payload, error) {
	newPayload := make([]ledger.Payload, 0, len(payload))
	for _, p := range payload {
		if len(p.Value) > 0 {
			newPayload = append(newPayload, p)
		}
	}
	return newPayload, nil
}
