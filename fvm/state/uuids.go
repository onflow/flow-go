package fvm

import (
	"encoding/binary"

	"github.com/dapperlabs/flow-go/fvm/state"
	"github.com/dapperlabs/flow-go/utils/slices"
)

type UUIDs struct {
	ledger Ledger
}

func NewUUIDs(ledger Ledger) *UUIDs {
	return &UUIDs{
		ledger: ledger,
	}
}

func (u *UUIDs) GetUUID() (uint64, error) {
	stateBytes, err := u.ledger.Get(state.fullKeyHash("", "", state.keyUUID))
	if err != nil {
		return 0, err
	}
	bytes := slices.EnsureByteSliceSize(stateBytes, 8)

	return binary.BigEndian.Uint64(bytes), nil
}

func (u *UUIDs) SetUUID(uuid uint64) {
	bytes := make([]byte, 8)
	binary.BigEndian.PutUint64(bytes, uuid)
	u.ledger.Set(state.fullKeyHash("", "", state.keyUUID), bytes)
}
