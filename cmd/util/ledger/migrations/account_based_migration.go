package migrations

import (
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
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
	id, err := KeyToRegisterID(k)
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
	MigratePayloads(account string, payloads []ledger.Payload, pathFinderVersion uint8) ([]ledger.Payload, error)
}

// MigrateByAccount teaks a migrator function and all the payloads, and return the migrated payloads
func MigrateByAccount(migrator AccountMigrator, allPayloads []ledger.Payload, pathFinderVersion uint8) (
	[]ledger.Payload, []ledger.Path, error) {
	groups := &PayloadGroup{
		NonAccountPayloads: make([]ledger.Payload, 0),
		Accounts:           make(map[string][]ledger.Payload),
	}

	log.Info().Msgf("start grouping for a total of %v payloads", len(allPayloads))

	var err error
	logGrouping := util.LogProgress("grouping payload", len(allPayloads), &log.Logger)
	for i, payload := range allPayloads {
		groups, err = PayloadGrouping(groups, payload)
		if err != nil {
			return nil, nil, err
		}
		logGrouping(i)
	}

	log.Info().Msgf("finish grouping for payloads by account: %v groups in total, %v NonAccountPayloads",
		len(groups.Accounts), len(groups.NonAccountPayloads))

	// migrate the payloads under accounts
	// migrated, paths, err := MigrateGroupSequentially(migrator, groups.Accounts, pathFinderVersion)
	migrated, paths, err := MigrateGroupConcurrently(migrator, groups.Accounts, pathFinderVersion)

	if err != nil {
		return nil, nil, fmt.Errorf("could not migrate group: %w", err)
	}

	log.Info().Msgf("finished migrating payloads for %v account", len(groups.Accounts))

	nonAccountPaths, err := pathfinder.PathsFromPayloads(groups.NonAccountPayloads, pathFinderVersion)
	if err != nil {
		return nil, nil, fmt.Errorf("could not find paths for non account payloads: %w", err)
	}

	// add the non accounts which don't need to be migrated
	migrated = append(migrated, groups.NonAccountPayloads...)
	paths = append(paths, nonAccountPaths...)

	log.Info().Msgf("finished migrating all account based payloads, total migrated payloads: %v", len(migrated))

	return migrated, paths, nil
}

// MigrateGroupSequentially migrate the payloads in the given payloadsByAccount map which
// using the migrator
func MigrateGroupSequentially(
	migrator AccountMigrator,
	payloadsByAccount map[string][]ledger.Payload,
	pathFinderVersion uint8,
) (
	[]ledger.Payload, []ledger.Path, error) {

	logAccount := util.LogProgress("processing account group", len(payloadsByAccount), &log.Logger)

	i := 0
	migrated := make([]ledger.Payload, 0)
	migratedPaths := make([]ledger.Path, 0)
	for address, payloads := range payloadsByAccount {
		accountMigrated, err := migrator.MigratePayloads(address, payloads, pathFinderVersion)
		if err != nil {
			return nil, nil, fmt.Errorf("could not migrate for account address %v: %w", address, err)
		}

		migrated = append(migrated, accountMigrated...)
		paths, err := pathfinder.PathsFromPayloads(accountMigrated, pathFinderVersion)
		if err != nil {
			return nil, nil, fmt.Errorf("can not find paths for migrated payload: %w", err)
		}
		migratedPaths = append(migratedPaths, paths...)
		logAccount(i)
		i++
	}

	return migrated, migratedPaths, nil
}

type jobMigrateAccountGroup struct {
	Account  string
	Payloads []ledger.Payload
}

type resultMigrating struct {
	Migrated []ledger.Payload
	Err      error
}

// MigrateGroupConcurrently migrate the payloads in the given payloadsByAccount map which
// using the migrator
// It's similar to MigrateGroupSequentially, except it will migrate different groups concurrently
func MigrateGroupConcurrently(
	migrator AccountMigrator,
	payloadsByAccount map[string][]ledger.Payload,
	pathFinderVersion uint8,
) (
	[]ledger.Payload, []ledger.Path, error) {
	nWorker := 10

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

	resultCh := make(chan *resultMigrating)
	for i := 0; i < int(nWorker); i++ {
		go func() {
			for job := range jobs {
				accountMigrated, err := migrator.MigratePayloads(job.Account, job.Payloads, pathFinderVersion)
				resultCh <- &resultMigrating{
					Migrated: accountMigrated,
					Err:      err,
				}
			}
		}()
	}

	// read job results
	logAccount := util.LogProgress("processing account group", len(payloadsByAccount), &log.Logger)

	migrated := make([]ledger.Payload, 0)
	migratedPaths := make([]ledger.Path, 0)

	for i := 0; i < len(payloadsByAccount); i++ {
		result := <-resultCh
		if result.Err != nil {
			return nil, nil, fmt.Errorf("fail to migrate payload: %w", result.Err)
		}

		accountMigrated := result.Migrated
		migrated = append(migrated, accountMigrated...)
		var err error
		paths, err := pathfinder.PathsFromPayloads(accountMigrated, pathFinderVersion)
		if err != nil {
			return nil, nil, fmt.Errorf("can not find paths for migrated payload: %w", err)
		}
		migratedPaths = append(migratedPaths, paths...)
		logAccount(i)
	}

	return migrated, migratedPaths, nil
}
