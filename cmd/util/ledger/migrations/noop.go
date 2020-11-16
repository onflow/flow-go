package migrations

import (
	"github.com/onflow/flow-go/ledger"
)

func NoOpMigration(p []ledger.Payload) ([]ledger.Payload, error) {
	return p, nil
}
