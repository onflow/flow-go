package state

import (
	"encoding/binary"

	"github.com/onflow/flow-go/utils/slices"
)

const keyUUID = "uuid"

type UUIDs struct {
	state *State
}

func NewUUIDs(state *State) *UUIDs {
	return &UUIDs{
		state: state,
	}
}

func (u *UUIDs) GetUUID() (uint64, error) {
	stateBytes, err := u.state.Get("", "", keyUUID)
	if err != nil {
		return 0, err
	}
	bytes := slices.EnsureByteSliceSize(stateBytes, 8)

	return binary.BigEndian.Uint64(bytes), nil
}

func (u *UUIDs) SetUUID(uuid uint64) {
	bytes := make([]byte, 8)
	binary.BigEndian.PutUint64(bytes, uuid)
	err := u.state.Set("", "", keyUUID, bytes)
	// TODO return the error instead
	if err != nil {
		panic(err)
	}
}
