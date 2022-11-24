package migrations

import (
	"fmt"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
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
type AccountMigrator func(account string, payloads []ledger.Payload) ([]ledger.Payload, error)

// MigrateByAccount teaks a migrator function and all the payloads, and return the migrated payloads
func MigrateByAccount(migrator AccountMigrator, allPayloads []ledger.Payload) ([]ledger.Payload, error) {
	groups := &PayloadGroup{
		NonAccountPayloads: make([]ledger.Payload, 0),
		Accounts:           make(map[string][]ledger.Payload),
	}

	var err error
	for _, payload := range allPayloads {
		groups, err = PayloadGrouping(groups, payload)
		if err != nil {
			return nil, err
		}
	}

	// migrate the payloads under accounts
	migrated, err := MigrateGroupSequentially(migrator, groups.Accounts)

	if err != nil {
		return nil, fmt.Errorf("could not migrate group: %w", err)
	}

	// add the non accounts which don't need to be migrated
	withNonAccount := append(migrated, groups.NonAccountPayloads...)

	return withNonAccount, nil
}

// MigrateGroupSequentially migrate the payloads in the given payloadsByAccount map which
// using the migrator
func MigrateGroupSequentially(migrator AccountMigrator, payloadsByAccount map[string][]ledger.Payload) (
	[]ledger.Payload, error) {
	migrated := make([]ledger.Payload, 0)
	for address, payloads := range payloadsByAccount {
		accountMigrated, err := migrator(address, payloads)
		if err != nil {
			return nil, fmt.Errorf("could not migrate for account address %v: %w", address, err)
		}

		migrated = append(migrated, accountMigrated...)
	}

	return migrated, nil
}

// MigrateGroupConcurrently migrate the payloads in the given payloadsByAccount map which
// using the migrator
// It's similar to MigrateGroupSequentially, except it will migrate different groups concurrently
func MigrateGroupConcurrently(migrator AccountMigrator, payloadsByAccount map[string][]ledger.Payload) (
	[]ledger.Payload, error) {
	panic("TO BE IMPLEMENTED")
}
