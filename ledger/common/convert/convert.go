package convert

import (
	"fmt"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

const (
	KeyPartOwner = uint16(0)
	// @deprecated - controller was used only by the very first
	// version of cadence for access controll which was retired later on
	// KeyPartController = uint16(1)
	KeyPartKey = uint16(2)
)

// UnexpectedLedgerKeyFormat is returned when a ledger key is not in the expected format
var UnexpectedLedgerKeyFormat = fmt.Errorf("unexpected ledger key format")

// LedgerKeyToRegisterID converts a ledger key to a register id
// returns an UnexpectedLedgerKeyFormat error if the key is not in the expected format
func LedgerKeyToRegisterID(key ledger.Key) (flow.RegisterID, error) {
	if len(key.KeyParts) != 2 ||
		key.KeyParts[0].Type != KeyPartOwner ||
		key.KeyParts[1].Type != KeyPartKey {
		return flow.RegisterID{}, fmt.Errorf("ledger key %s: %w", key.String(), UnexpectedLedgerKeyFormat)
	}

	return flow.NewRegisterID(
		string(key.KeyParts[0].Value),
		string(key.KeyParts[1].Value),
	), nil
}

// RegisterIDToLedgerKey converts a register id to a ledger key
func RegisterIDToLedgerKey(registerID flow.RegisterID) ledger.Key {
	newKey := ledger.Key{}
	newKey.KeyParts = []ledger.KeyPart{
		{
			Type:  KeyPartOwner,
			Value: []byte(registerID.Owner),
		},
		{
			Type:  KeyPartKey,
			Value: []byte(registerID.Key),
		},
	}
	return newKey
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
