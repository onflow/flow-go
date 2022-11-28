package migrations

import "github.com/onflow/flow-go/ledger"

func MigrateAccountUsage(payloads []ledger.Payload, pathFinderVersion uint8) ([]ledger.Payload, []ledger.Path, error) {
	return MigrateByAccount(AccountUsageMigrator, payloads, pathFinderVersion)
}

func AccountUsageMigrator(account string, payloads []ledger.Payload) ([]ledger.Payload, error) {
	return payloads, nil
}
