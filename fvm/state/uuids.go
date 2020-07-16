package state

import (
	"encoding/binary"

	"github.com/dapperlabs/flow-go/utils/slices"
)

const keyUUID = "uuid"

type UUIDs struct {
	ledger Ledger
}

func NewUUIDs(ledger Ledger) *UUIDs {
	return &UUIDs{
		ledger: ledger,
	}
}

func (u *UUIDs) GetUUID() (uint64, error) {
	stateBytes, err := u.ledger.Get(RegisterID("", "", keyUUID))
	if err != nil {
		return 0, err
	}
	bytes := slices.EnsureByteSliceSize(stateBytes, 8)

	return binary.BigEndian.Uint64(bytes), nil
}

func (u *UUIDs) SetUUID(uuid uint64) {
	bytes := make([]byte, 8)
	binary.BigEndian.PutUint64(bytes, uuid)
	u.ledger.Set(RegisterID("", "", keyUUID), bytes)
}
