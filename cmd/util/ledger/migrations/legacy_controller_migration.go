package migrations

import (
	"bytes"
	"encoding/hex"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/execution/state"
	fvmState "github.com/onflow/flow-go/fvm/state"
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
	newPayloads := make([]ledger.Payload, len(payload))
	for i, p := range payload {
		k, err := p.Key()
		if err != nil {
			return nil, err
		}
		owner := k.KeyParts[0].Value
		controller := k.KeyParts[1].Value
		key := k.KeyParts[2].Value

		if len(controller) > 0 {
			if bytes.Equal(owner, controller) &&
				string(key) != KeyPublicKeyCount && //  case - public key count
				!bytes.HasPrefix(key, []byte("public_key_")) && // case - public keys
				string(key) != fvmState.KeyContractNames && // case - contract names
				!bytes.HasPrefix(key, []byte(fvmState.KeyCode)) { // case - contracts
				lc.Logger.Warn().Msgf("found an unexpected new case of non-empty controller use: %s, %s, %s",
					hex.EncodeToString(owner),
					hex.EncodeToString(controller),
					hex.EncodeToString(key),
				)
			}
		}
		newKey := ledger.NewKey([]ledger.KeyPart{
			ledger.NewKeyPart(state.KeyPartOwner, owner),
			ledger.NewKeyPart(state.KeyPartKey, key),
		})
		newPayloads[i] = *ledger.NewPayload(newKey, p.Value())
	}
	return newPayloads, nil
}
