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

func AccountUsageMigrator(account string, payloads []ledger.Payload, pathFinderVersion uint8) ([]ledger.Payload, error) {
	// iteration through each payload
	// and find the one payload that is for account status
	// with the key state.AccountStatusKey, which will return a AccountStatus obj
	// and sum up the size of all payloads.
	// the size of a payload can be calculated by constructing
	// a StorageInteractionKey and call GetStorageKeyValueSizeForTesting
	// and update account storage size by updating the original AccountStatus obj
	// with SetStorageUsed method
	// use toBytes to get new value for the payload
	// and create a new payload with NewPayload to replace the old payload with the new value

	var status *environment.AccountStatus
	var statusIndex int
	totalSize := uint64(0)
	for i, payload := range payloads {
		key, err := payload.Key()
		if err != nil {
			return nil, err
		}
		if string(key.KeyParts[1].Value) == state.AccountStatusKey {
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

func newPayloadWithValue(payload ledger.Payload, value ledger.Value) (ledger.Payload, error) {
	key, err := payload.Key()
	if err != nil {
		return ledger.Payload{}, err
	}
	newPayload := ledger.NewPayload(key, payload.Value())
	return *newPayload, nil
}
