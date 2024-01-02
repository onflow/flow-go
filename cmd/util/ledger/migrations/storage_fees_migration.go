package migrations

import (
	fvm "github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/ledger/common/utils"
	"github.com/onflow/flow-go/model/flow"
)

// iterates through registers keeping a map of register sizes
// after it has reached the end it add storage used and storage capacity for each address
func StorageFeesMigration(payload []ledger.Payload) ([]ledger.Payload, error) {
	storageUsed := make(map[string]uint64)
	newPayload := make([]ledger.Payload, len(payload))

	for i, p := range payload {
		err := incrementStorageUsed(p, storageUsed)
		if err != nil {
			return nil, err
		}
		newPayload[i] = p
	}

	for s, u := range storageUsed {
		// this is the storage used by the storage_used register we are about to add
		id := flow.NewRegisterID(
			flow.BytesToAddress([]byte(s)),
			"storage_used")
		storageUsedByStorageUsed := fvm.RegisterSize(id, make([]byte, 8))
		u = u + uint64(storageUsedByStorageUsed)

		newPayload = append(newPayload, *ledger.NewPayload(
			convert.RegisterIDToLedgerKey(id),
			utils.Uint64ToBinary(u),
		))
	}
	return newPayload, nil
}

func incrementStorageUsed(p ledger.Payload, used map[string]uint64) error {
	k, err := p.Key()
	if err != nil {
		return err
	}
	id, err := convert.LedgerKeyToRegisterID(k)
	if err != nil {
		return err
	}
	if len([]byte(id.Owner)) != flow.AddressLength {
		// not an address
		return nil
	}
	if _, ok := used[id.Owner]; !ok {
		used[id.Owner] = 0
	}
	used[id.Owner] = used[id.Owner] + uint64(registerSize(id, p))
	return nil
}

func registerSize(id flow.RegisterID, p ledger.Payload) int {
	return fvm.RegisterSize(id, p.Value())
}
