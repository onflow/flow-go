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
func PayloadToAccount(p ledger.Payload) (string, DecodedPayload, bool, error) {
	k, err := p.Key()
	if err != nil {
		return "", DecodedPayload{}, false, fmt.Errorf("could not find key for payload: %w", err)
	}
	id, err := KeyToRegisterID(k)
	if err != nil {
		return "", DecodedPayload{}, false, fmt.Errorf("error converting key to register ID")
	}
	if len([]byte(id.Owner)) != flow.AddressLength {
		return "", DecodedPayload{}, false, nil
	}
	return id.Owner, DecodedPayload{Key: k, Payload: p}, true, nil
}

type DecodedPayload struct {
	Payload ledger.Payload
	Key     ledger.Key
}

// PayloadGroup groups payloads by account.
// For global payloads, it's stored under NonAccountPayloads field
type PayloadGroup struct {
	NonAccountPayloads []ledger.Payload
	Accounts           map[string][]DecodedPayload
}

// PayloadGrouping is a reducer function that adds the given payload to the corresponding
// group under its account
func PayloadGrouping(groups *PayloadGroup, payload ledger.Payload) (*PayloadGroup, error) {
	address, decoded, isAccount, err := PayloadToAccount(payload)
	if err != nil {
		return nil, err
	}

	if isAccount {
		groups.Accounts[address] = append(groups.Accounts[address], decoded)
	} else {
		groups.NonAccountPayloads = append(groups.NonAccountPayloads, payload)
	}

	return groups, nil
}

// AccountMigrator takes all the payloads that belong to the given account
// and return the migrated payloads
type AccountMigrator func(account string, payloads []ledger.Payload) ([]ledger.Payload, error)

// MigrateByAccount teaks a migrator function and all the payloads, and return the migrated payloads
func MigrateByAccount(migrator AccountMigrator, allPayloads []ledger.Payload, pathFinderVersion uint8) (
	[]ledger.Payload, []ledger.Path, error) {
	groups := &PayloadGroup{
		NonAccountPayloads: make([]ledger.Payload, 0),
		Accounts:           make(map[string][]DecodedPayload),
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

	log.Info().Msgf("finish grouping for payloads, %v groups in total, %v NonAccountPayloads",
		len(groups.Accounts), len(groups.NonAccountPayloads))

	// migrate the payloads under accounts
	migrated, paths, err := MigrateGroupSequentially(migrator, groups.Accounts, pathFinderVersion)

	if err != nil {
		return nil, nil, fmt.Errorf("could not migrate group: %w", err)
	}

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
	payloadsByAccount map[string][]DecodedPayload,
	pathFinderVersion uint8,
) (
	[]ledger.Payload, []ledger.Path, error) {

	logAccount := util.LogProgress("processing account group", len(payloadsByAccount), &log.Logger)

	i := 0
	migrated := make([]ledger.Payload, 0)
	migratedPaths := make([]ledger.Path, 0)
	for address, decoded := range payloadsByAccount {
		payloads := make([]ledger.Payload, len(decoded))
		for i, d := range decoded {
			payloads[i] = d.Payload
		}

		accountMigrated, err := migrator(address, payloads)
		if err != nil {
			return nil, nil, fmt.Errorf("could not migrate for account address %v: %w", address, err)
		}

		migrated = append(migrated, accountMigrated...)
		migratedPaths, err = pathfinder.PathsFromPayloads(accountMigrated, pathFinderVersion)
		if err != nil {
			return nil, nil, fmt.Errorf("can not find paths for migrated payload: %w", err)
		}
		logAccount(i)
		i++
	}

	return migrated, migratedPaths, nil
}

// MigrateGroupConcurrently migrate the payloads in the given payloadsByAccount map which
// using the migrator
// It's similar to MigrateGroupSequentially, except it will migrate different groups concurrently
func MigrateGroupConcurrently(migrator AccountMigrator, payloadsByAccount map[string][]ledger.Payload) (
	[]ledger.Payload, error) {
	// logAccount := util.LogProgress("processing account group", len(payloadsByAccount), &log.Logger)
	panic("TO IMPLEMENT")
}
