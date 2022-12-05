package migrations

import (
	"fmt"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
)

func MigrateAccountUsage(payloads []ledger.Payload, pathFinderVersion uint8) ([]ledger.Payload, []ledger.Path, error) {
	return MigrateByAccount(AccountUsageMigrator, payloads, pathFinderVersion)
}

func KeyToStorageInteractionKey(key ledger.Key) (meter.StorageInteractionKey, error) {
	id, err := KeyToRegisterID(key)
	if err != nil {
		return meter.StorageInteractionKey{}, err
	}
	return meter.StorageInteractionKey{
		Owner: id.Owner,
		Key:   id.Key,
	}, nil
}

func payloadSize(key ledger.Key, payload ledger.Payload) (uint64, error) {
	ik, err := KeyToStorageInteractionKey(key)
	if err != nil {
		return 0, err
	}

	return meter.GetStorageKeyValueSizeForTesting(ik, payload.Value()), nil
}

func isAccountKey(key ledger.Key) bool {
	return string(key.KeyParts[1].Value) == state.AccountStatusKey
}

// AccountUsageMigrator iterate through each payload, and calculate the storage usage
// and update the accoutns status with the updated storage usage
func AccountUsageMigrator(account string, payloads []ledger.Payload, pathFinderVersion uint8) ([]ledger.Payload, error) {
	var status *environment.AccountStatus
	var statusIndex int
	totalSize := uint64(0)
	for i, payload := range payloads {
		key, err := payload.Key()
		if err != nil {
			return nil, err
		}
		if isAccountKey(key) {
			statusIndex = i
			status, err = environment.AccountStatusFromBytes(payload.Value())
			if err != nil {
				return nil, fmt.Errorf("could not parse account status: %w", err)
			}

		}

		size, err := payloadSize(key, payload)
		if err != nil {
			return nil, err
		}
		totalSize += size
	}

	if status == nil {
		return nil, fmt.Errorf("could not find account status for account %v", account)
	}

	// update storage used
	status.SetStorageUsed(totalSize)

	newValue := status.ToBytes()
	newPayload, err := newPayloadWithValue(payloads[statusIndex], newValue)
	if err != nil {
		return nil, fmt.Errorf("cannot create new payload with value: %w", err)
	}

	payloads[statusIndex] = newPayload

	return payloads, nil
}

// newPayloadWithValue returns a new payload with the key from the given payload, and
// the value from the argument
func newPayloadWithValue(payload ledger.Payload, value ledger.Value) (ledger.Payload, error) {
	key, err := payload.Key()
	if err != nil {
		return ledger.Payload{}, err
	}
	newPayload := ledger.NewPayload(key, payload.Value())
	return *newPayload, nil
}
