package migrations

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

// MigrateAccountUsage iterates through each payload, and calculate the storage usage
// and update the accoutns status with the updated storage usage
func MigrateAccountUsage(log zerolog.Logger, nWorker int) func([]*ledger.Payload) ([]*ledger.Payload, error) {
	return CreateAccountBasedMigration(
		log,
		func(allPayloads []*ledger.Payload, nWorker int) (AccountMigrator, error) {
			return AccountUsageMigrator{}, nil
		},
		nWorker,
	)
}

func payloadSize(key ledger.Key, payload *ledger.Payload) (uint64, error) {
	id, err := util.KeyToRegisterID(key)
	if err != nil {
		return 0, err
	}

	return uint64(registerSize(id, payload)), nil
}

func isAccountKey(key ledger.Key) bool {
	return string(key.KeyParts[1].Value) == flow.AccountStatusKey
}

type AccountUsageMigrator struct{}

func (m AccountUsageMigrator) MigratePayloads(
	ctx context.Context,
	address common.Address,
	payloads []*ledger.Payload,
) ([]*ledger.Payload, error) {
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

	err := compareUsage(status, totalSize)
	if err != nil {
		log.Error().Msgf("%v", err)
	}

	if status == nil {
		return nil, fmt.Errorf("could not find account status for account %v", address.Hex())
	}

	// update storage used
	status.SetStorageUsed(totalSize)

	newValue := status.ToBytes()
	newPayload, err := newPayloadWithValue(payloads[statusIndex], newValue)
	if err != nil {
		return nil, fmt.Errorf("cannot create new payload with value: %w", err)
	}

	payloads[statusIndex] = &newPayload

	return payloads, nil
}

func compareUsage(status *environment.AccountStatus, totalSize uint64) error {
	oldSize := status.StorageUsed()
	if oldSize != totalSize {
		return fmt.Errorf("old size: %v, new size: %v", oldSize, totalSize)
	}
	return nil
}

// newPayloadWithValue returns a new payload with the key from the given payload, and
// the value from the argument
func newPayloadWithValue(payload *ledger.Payload, value ledger.Value) (ledger.Payload, error) {
	key, err := payload.Key()
	if err != nil {
		return ledger.Payload{}, err
	}
	newPayload := ledger.NewPayload(key, payload.Value())
	return *newPayload, nil
}
