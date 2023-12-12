package migrations

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/environment"
	fvm "github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
)

// AccountUsageMigrator iterates through each payload, and calculate the storage usage
// and update the accounts status with the updated storage usage
type AccountUsageMigrator struct {
	log zerolog.Logger
}

var _ AccountBasedMigration = &AccountUsageMigrator{}

func (m *AccountUsageMigrator) InitMigration(
	log zerolog.Logger,
	_ []*ledger.Payload,
	_ int,
) error {
	m.log = log.With().Str("component", "AccountUsageMigrator").Logger()
	return nil
}

const oldAccountStatusSize = 25

func (m *AccountUsageMigrator) MigrateAccount(
	_ context.Context,
	address common.Address,
	payloads []*ledger.Payload,
) ([]*ledger.Payload, error) {

	var status *environment.AccountStatus
	var statusIndex int
	actualSize := uint64(0)
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
		actualSize += size
	}

	if status == nil {
		return nil, fmt.Errorf("could not find account status for account %v", address.Hex())
	}

	isOldVersionOfStatusRegister := len(payloads[statusIndex].Value()) == oldAccountStatusSize

	same := m.compareUsage(isOldVersionOfStatusRegister, status, actualSize)
	if same {
		// there is no problem with the usage, return
		return payloads, nil
	}

	if isOldVersionOfStatusRegister {
		// size will grow by 8 bytes because of the on-the-fly
		// migration of account status in AccountStatusFromBytes
		actualSize += 8
	}

	// update storage used
	status.SetStorageUsed(actualSize)

	newValue := status.ToBytes()
	newPayload, err := newPayloadWithValue(payloads[statusIndex], newValue)
	if err != nil {
		return nil, fmt.Errorf("cannot create new payload with value: %w", err)
	}

	payloads[statusIndex] = newPayload

	return payloads, nil
}

func (m *AccountUsageMigrator) compareUsage(
	isOldVersionOfStatusRegister bool,
	status *environment.AccountStatus,
	actualSize uint64,
) bool {
	oldSize := status.StorageUsed()
	if isOldVersionOfStatusRegister {
		// size will be reported as 8 bytes larger than the actual size due to on-the-fly
		// migration of account status in AccountStatusFromBytes
		oldSize -= 8
	}

	if oldSize != actualSize {
		m.log.Info().
			Uint64("old_size", oldSize).
			Uint64("new_size", actualSize).
			Msg("account storage used usage mismatch")
		return false
	}
	return true
}

// newPayloadWithValue returns a new payload with the key from the given payload, and
// the value from the argument
func newPayloadWithValue(payload *ledger.Payload, value ledger.Value) (*ledger.Payload, error) {
	key, err := payload.Key()
	if err != nil {
		return &ledger.Payload{}, err
	}
	newPayload := ledger.NewPayload(key, value)
	return newPayload, nil
}

func registerSize(id flow.RegisterID, p *ledger.Payload) int {
	return fvm.RegisterSize(id, p.Value())
}

func payloadSize(key ledger.Key, payload *ledger.Payload) (uint64, error) {
	id, err := convert.LedgerKeyToRegisterID(key)
	if err != nil {
		return 0, err
	}

	return uint64(registerSize(id, payload)), nil
}

func isAccountKey(key ledger.Key) bool {
	return string(key.KeyParts[1].Value) == flow.AccountStatusKey
}
