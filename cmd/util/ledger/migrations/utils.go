package migrations

import (
	"fmt"

	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

func keyToRegisterID(key ledger.Key) (flow.RegisterID, error) {
	if len(key.KeyParts) != 3 ||
		key.KeyParts[0].Type != state.KeyPartOwner ||
		key.KeyParts[1].Type != state.KeyPartController ||
		key.KeyParts[2].Type != state.KeyPartKey {
		return flow.RegisterID{}, fmt.Errorf("key not in expected format %s", key.String())
	}

	return flow.NewRegisterID(
		string(key.KeyParts[0].Value),
		string(key.KeyParts[1].Value),
		string(key.KeyParts[2].Value),
	), nil
}

func registerIDToKey(registerID flow.RegisterID) ledger.Key {
	newKey := ledger.Key{}
	newKey.KeyParts = []ledger.KeyPart{
		{
			Type:  state.KeyPartOwner,
			Value: []byte(registerID.Owner),
		},
		{
			Type:  state.KeyPartController,
			Value: []byte(registerID.Controller),
		},
		{
			Type:  state.KeyPartKey,
			Value: []byte(registerID.Key),
		},
	}
	return newKey
}
