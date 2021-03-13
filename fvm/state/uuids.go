package state

import (
	"encoding/binary"
	"fmt"

	"github.com/onflow/flow-go/utils/slices"
)

const keyUUID = "uuid"

type UUIDGenerator struct {
	stateHolder *StateHolder
}

func NewUUIDGenerator(stateHolder *StateHolder) *UUIDGenerator {
	return &UUIDGenerator{
		stateHolder: stateHolder,
	}
}

func (u *UUIDGenerator) GetUUID() (uint64, error) {
	stateBytes, err := u.stateHolder.State().Get("", "", keyUUID)
	if err != nil {
		return 0, err
	}
	bytes := slices.EnsureByteSliceSize(stateBytes, 8)

	return binary.BigEndian.Uint64(bytes), nil
}

func (u *UUIDGenerator) SetUUID(uuid uint64) error {
	bytes := make([]byte, 8)
	binary.BigEndian.PutUint64(bytes, uuid)
	return u.stateHolder.State().Set("", "", keyUUID, bytes)
}

func (u *UUIDGenerator) GenerateUUID() (uint64, error) {
	uuid, err := u.GetUUID()
	if err != nil {
		return 0, fmt.Errorf("cannot get UUID: %w", err)
	}

	err = u.SetUUID(uuid + 1)
	if err != nil {
		return 0, fmt.Errorf("cannot set UUID: %w", err)
	}
	return uuid, nil
}
