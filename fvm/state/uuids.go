package state

import (
	"encoding/binary"

	"github.com/onflow/flow-go/utils/slices"
)

const keyUUID = "uuid"

type UUIDs struct {
	stateManager *StateManager
}

func NewUUIDs(stateManager *StateManager) *UUIDs {
	return &UUIDs{
		stateManager: stateManager,
	}
}

func (u *UUIDs) GetUUID() (uint64, error) {
	stateBytes, err := u.stateManager.State().Get("", "", keyUUID)
	if err != nil {
		return 0, err
	}
	bytes := slices.EnsureByteSliceSize(stateBytes, 8)

	return binary.BigEndian.Uint64(bytes), nil
}

func (u *UUIDs) SetUUID(uuid uint64) {
	bytes := make([]byte, 8)
	binary.BigEndian.PutUint64(bytes, uuid)
	err := u.stateManager.State().Set("", "", keyUUID, bytes)
	// TODO return the error instead
	if err != nil {
		panic(err)
	}
}
