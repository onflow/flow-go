package migrations

import (
	"fmt"
	"github.com/onflow/flow-go/engine/execution/state"
	fvm "github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/utils"
	"github.com/onflow/flow-go/model/flow"
)

const (
	sizeOfStorageRegisters = 16
)

// Migration assumes that payload containd all existing registers.
// iterates through registers keeping a map of register sizes
// after it has reached the end it add storage used and storage capacity for each address
func Migration(payload []ledger.Payload) ([]ledger.Payload, error) {
	storageUsed := make(map[string]uint64, 0)
	newPayload := make([]ledger.Payload, len(payload))
	
	for i, p := range payload {
		err := process(p, storageUsed)
		if err != nil {
			return nil, err
		}
		newPayload[i] = p
	}

	for s, u := range storageUsed {
		u = u + sizeOfStorageRegisters
		capacity := ((u / 500_000) + 2) * 500_000 // at least 500kb more than currently stored; 1MB if nothing stored

		newPayload = append(newPayload, ledger.Payload{
			Key:   makeKey(s, "storage_used"),
			Value: utils.Uint64ToBinary(u),
		}, ledger.Payload{
			Key:   makeKey(s, "storage_capacity"),
			Value: utils.Uint64ToBinary(capacity),
		})
	}
	return newPayload, nil
}

func makeKey(owner string, key string) ledger.Key {
	newKey := ledger.Key{}
	newKey.KeyParts = make([]ledger.KeyPart, 3)
	newKey.KeyParts[0] = ledger.KeyPart{
		Type:  state.KeyPartOwner,
		Value: []byte(owner),
	}
	newKey.KeyParts[1] = ledger.KeyPart{
		Type:  state.KeyPartController,
		Value: []byte(""),
	}
	newKey.KeyParts[2] = ledger.KeyPart{
		Type:  state.KeyPartKey,
		Value: []byte(key),
	}
	return newKey
}

func keyToRegisterId(key ledger.Key) (flow.RegisterID, error) {
	if len(key.KeyParts) != 3 ||
		key.KeyParts[0].Type != state.KeyPartOwner ||
		key.KeyParts[1].Type != state.KeyPartController ||
		key.KeyParts[2].Type != state.KeyPartKey {
		return flow.RegisterID{}, fmt.Errorf("key not in expected format %s", key.String())
	}

	return flow.NewRegisterID(string(key.KeyParts[0].Value), string(key.KeyParts[1].Value), string(key.KeyParts[2].Value)), nil
}

func process(p ledger.Payload, used map[string]uint64) error {
	id, err := keyToRegisterId(p.Key)
	if err != nil {
		return err
	}
	if len([]byte(id.Owner)) != flow.AddressLength {
		// not an address
		return nil
	}

	used[id.Owner] = used[id.Owner] + uint64(registerSize(id, p))
	return nil
}

func registerSize(id flow.RegisterID, p ledger.Payload) int {
	address := flow.BytesToAddress([]byte(id.Owner))
	isController := len(id.Controller) > 0
	key := id.Key
	return fvm.RegisterSize(address, isController, key, p.Value)
}
