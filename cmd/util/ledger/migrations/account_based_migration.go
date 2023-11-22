package migrations

import (
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/util"
)

// PayloadToAccount takes a payload and return:
// - (address, true, nil) if the payload is for an account, the account address is returned
// - ("", false, nil) if the payload is not for an account
// - ("", false, err) if running into any exception
func PayloadToAccount(p ledger.Payload) (string, bool, error) {
	k, err := p.Key()
	if err != nil {
		return "", false, fmt.Errorf("could not find key for payload: %w", err)
	}
	id, err := convert.LedgerKeyToRegisterID(k)
	if err != nil {
		return "", false, fmt.Errorf("error converting key to register ID")
	}
	if len([]byte(id.Owner)) != flow.AddressLength {
		return "", false, nil
	}
	return id.Owner, true, nil
}

// PayloadGroup groups payloads by account.
// For global payloads, it's stored under NonAccountPayloads field
type PayloadGroup struct {
	NonAccountPayloads []ledger.Payload
	Accounts           map[string][]ledger.Payload
}

// PayloadGrouping is a reducer function that adds the given payload to the corresponding
// group under its account
func PayloadGrouping(groups *PayloadGroup, payload ledger.Payload) (*PayloadGroup, error) {
	address, isAccount, err := PayloadToAccount(payload)
	if err != nil {
		return nil, err
	}

	if isAccount {
		groups.Accounts[address] = append(groups.Accounts[address], payload)
	} else {
		groups.NonAccountPayloads = append(groups.NonAccountPayloads, payload)
	}

	return groups, nil
}

// AccountMigrator takes all the payloads that belong to the given account
// and return the migrated payloads
type AccountMigrator interface {
	MigratePayloads(account string, payloads []ledger.Payload) ([]ledger.Payload, error)
}

// MigrateByAccount teaks a migrator function and all the payloads, and return the migrated payloads
func MigrateByAccount(migrator AccountMigrator, allPayloads []ledger.Payload, nWorker int) (
	[]ledger.Payload, error) {
	groups := &PayloadGroup{
		NonAccountPayloads: make([]ledger.Payload, 0),
		Accounts:           make(map[string][]ledger.Payload),
	}

	log.Info().Msgf("start grouping for a total of %v payloads", len(allPayloads))

	var err error
	logGrouping := util.LogProgress("grouping payload", len(allPayloads), log.Logger)
	for i, payload := range allPayloads {
		groups, err = PayloadGrouping(groups, payload)
		if err != nil {
			return nil, err
		}
		logGrouping(i)
	}

	log.Info().Msgf("finish grouping for payloads by account: %v groups in total, %v NonAccountPayloads",
		len(groups.Accounts), len(groups.NonAccountPayloads))

	// migrate the payloads under accounts
	migrated, err := MigrateGroupConcurrently(migrator, groups.Accounts, nWorker)

	if err != nil {
		return nil, fmt.Errorf("could not migrate group: %w", err)
	}

	log.Info().Msgf("finished migrating payloads for %v account", len(groups.Accounts))

	// add the non accounts which don't need to be migrated
	migrated = append(migrated, groups.NonAccountPayloads...)

	log.Info().Msgf("finished migrating all account based payloads, total migrated payloads: %v", len(migrated))

	return migrated, nil
}

// MigrateGroupSequentially migrate the payloads in the given payloadsByAccount map which
// using the migrator
func MigrateGroupSequentially(
	migrator AccountMigrator,
	payloadsByAccount map[string][]ledger.Payload,
) (
	[]ledger.Payload, error) {

	logAccount := util.LogProgress("processing account group", len(payloadsByAccount), log.Logger)

	i := 0
	migrated := make([]ledger.Payload, 0)
	for address, payloads := range payloadsByAccount {
		accountMigrated, err := migrator.MigratePayloads(address, payloads)
		if err != nil {
			return nil, fmt.Errorf("could not migrate for account address %v: %w", address, err)
		}

		migrated = append(migrated, accountMigrated...)
		logAccount(i)
		i++
	}

	return migrated, nil
}

type jobMigrateAccountGroup struct {
	Account  string
	Payloads []ledger.Payload
}

type migrationResult struct {
	Migrated []ledger.Payload
	Err      error
}

// MigrateGroupConcurrently migrate the payloads in the given payloadsByAccount map which
// using the migrator
// It's similar to MigrateGroupSequentially, except it will migrate different groups concurrently
func MigrateGroupConcurrently(
	migrator AccountMigrator,
	payloadsByAccount map[string][]ledger.Payload,
	nWorker int,
) (
	[]ledger.Payload, error) {

	jobs := make(chan jobMigrateAccountGroup, len(payloadsByAccount))
	go func() {
		for account, payloads := range payloadsByAccount {
			jobs <- jobMigrateAccountGroup{
				Account:  account,
				Payloads: payloads,
			}
		}
		close(jobs)
	}()

	resultCh := make(chan *migrationResult)
	for i := 0; i < int(nWorker); i++ {
		go func() {
			for job := range jobs {
				accountMigrated, err := migrator.MigratePayloads(job.Account, job.Payloads)
				resultCh <- &migrationResult{
					Migrated: accountMigrated,
					Err:      err,
				}
			}
		}()
	}

	// read job results
	logAccount := util.LogProgress("processing account group", len(payloadsByAccount), log.Logger)

	migrated := make([]ledger.Payload, 0)

	for i := 0; i < len(payloadsByAccount); i++ {
		result := <-resultCh
		if result.Err != nil {
			return nil, fmt.Errorf("fail to migrate payload: %w", result.Err)
		}

		accountMigrated := result.Migrated
		migrated = append(migrated, accountMigrated...)
		logAccount(i)
	}

	return migrated, nil
}
