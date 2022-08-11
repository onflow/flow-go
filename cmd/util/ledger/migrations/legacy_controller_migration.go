package migrations

import (
	"bytes"
	"encoding/hex"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
)

// LegacyControllerMigration migrates previous registers
// that were using the legacy controller value (value is always set to the same owner value).
// the controller value should now all be empty and
// in future can be remove all together.
type LegacyControllerMigration struct {
	Logger zerolog.Logger
}

func (lc *LegacyControllerMigration) Migrate(payload []ledger.Payload) ([]ledger.Payload, error) {
	for _, p := range payload {
		owner := p.Key.KeyParts[0].Value
		controller := p.Key.KeyParts[1].Value
		key := p.Key.KeyParts[2].Value

		if len(controller) > 0 {
			if bytes.Equal(owner, controller) {
				//
				if string(key) == state.KeyPublicKeyCount || //  case - public key count
					bytes.HasPrefix(key, []byte("public_key_")) || // case - public keys
					string(key) == state.KeyContractNames || // case - contract names
					bytes.HasPrefix(key, []byte(state.KeyCode)) { // case - contracts
					p.Key.KeyParts[1].Value = []byte("")
					continue
				}
			}
			// else we have found an unexpected new case of non-empty controller use
			lc.Logger.Warn().Msgf("found an unexpected new case of non-empty controller use: %s, %s, %s",
				hex.EncodeToString(owner),
				hex.EncodeToString(controller),
				hex.EncodeToString(key),
			)
		}
	}
	return payload, nil
}
