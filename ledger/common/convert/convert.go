package convert

import (
	"fmt"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

// UnexpectedLedgerKeyFormat is returned when a ledger key is not in the expected format
var UnexpectedLedgerKeyFormat = fmt.Errorf("unexpected ledger key format")

// LedgerKeyToRegisterID converts a ledger key to a register id
// returns an UnexpectedLedgerKeyFormat error if the key is not in the expected format
func LedgerKeyToRegisterID(key ledger.Key) (flow.RegisterID, error) {
	parts := key.KeyParts
	if len(parts) != 2 ||
		parts[0].Type != ledger.KeyPartOwner ||
		parts[1].Type != ledger.KeyPartKey {
		return flow.RegisterID{}, fmt.Errorf("ledger key %s: %w", key.String(), UnexpectedLedgerKeyFormat)
	}

	return flow.NewRegisterID(
		flow.BytesToAddress(parts[0].Value),
		string(parts[1].Value),
	), nil
}

// RegisterIDToLedgerKey converts a register id to a ledger key
func RegisterIDToLedgerKey(registerID flow.RegisterID) ledger.Key {
	return ledger.Key{
		KeyParts: []ledger.KeyPart{
			{
				Type:  ledger.KeyPartOwner,
				Value: []byte(registerID.Owner),
			},
			{
				Type:  ledger.KeyPartKey,
				Value: []byte(registerID.Key),
			},
		},
	}
}

// PayloadToRegister converts a payload to a register id and value
func PayloadToRegister(payload *ledger.Payload) (flow.RegisterID, flow.RegisterValue, error) {
	key, err := payload.Key()
	if err != nil {
		return flow.RegisterID{}, flow.RegisterValue{}, fmt.Errorf("could not parse register key from payload: %w", err)
	}
	regID, err := LedgerKeyToRegisterID(key)
	if err != nil {
		return flow.RegisterID{}, flow.RegisterValue{}, fmt.Errorf("could not convert register key into register id: %w", err)
	}

	return regID, payload.Value(), nil
}
