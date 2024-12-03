package migrations

import (
	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
)

type RegistersMigration func(registersByAccount *registers.ByAccount) error

type NamedMigration struct {
	Name    string
	Migrate RegistersMigration
}
