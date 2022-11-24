package migrations

import "github.com/onflow/flow-go/ledger"

func MigrateAccountUsage(payloads []ledger.Payload) ([]ledger.Payload, error) {
	return MigrateByAccount(AccountUsageMigrator, payloads)
}

func AccountUsageMigrator(account string, payloads []ledger.Payload) ([]ledger.Payload, error) {
	return payloads, nil
}
