package migrations

import (
	"encoding/hex"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/rs/zerolog"
)

// AccountStatusMigration migrates previous registers under
// key of Exists which were used for checking existance of accounts.
// the new register AccountStatus also captures frozen and all future states
// of the accounts. Frozen state is used when an account is set
// by the network governance for furture investigation and prevents operations on the account until
// furthure investigation by the community.
// This migration assumes no account has been frozen until now, and would warn if
// find any account with frozen flags.
type AccountStatusMigration struct {
	Logger zerolog.Logger
}

func (as *AccountStatusMigration) Migrate(payload []ledger.Payload) ([]ledger.Payload, error) {
	newPayloads := make([]ledger.Payload, 0, len(payload))
	for _, p := range payload {
		owner := p.Key.KeyParts[0].Value
		controller := p.Key.KeyParts[1].Value
		key := p.Key.KeyParts[2].Value
		if len(controller) == 0 && string(key) == state.KeyExists {
			newPayload := p.DeepCopy()
			newPayload.Key.KeyParts[2].Value = []byte(state.KeyAccountStatus)
			newPayload.Value = state.NewAccountStatus().ToBytes()
			newPayloads = append(newPayloads, *newPayload)
			continue
		}
		if len(controller) == 0 && string(key) == state.KeyAccountFrozen {
			as.Logger.Warn().Msgf("frozen account found: %s", hex.EncodeToString(owner))
			continue
		}
		// else just append and continue
		newPayloads = append(newPayloads, p)
	}
	return newPayloads, nil
}
