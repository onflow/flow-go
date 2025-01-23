package migrations

import (
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/stdlib"

	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
)

type RegistersMigration func(registersByAccount *registers.ByAccount) error

type NamedMigration struct {
	Name    string
	Migrate RegistersMigration
}

var AllStorageMapDomains = []string{
	common.PathDomainStorage.Identifier(),
	common.PathDomainPrivate.Identifier(),
	common.PathDomainPublic.Identifier(),
	runtime.StorageDomainContract,
	stdlib.InboxStorageDomain,
	stdlib.CapabilityControllerStorageDomain,
	stdlib.PathCapabilityStorageDomain,
	stdlib.AccountCapabilityStorageDomain,
}

var allStorageMapDomainsSet = map[string]struct{}{}

func init() {
	for _, domain := range AllStorageMapDomains {
		allStorageMapDomainsSet[domain] = struct{}{}
	}
}
