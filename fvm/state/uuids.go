package state

import (
	"encoding/binary"
	"fmt"

	"github.com/onflow/flow-go/utils/slices"
)

const keyUUID = "uuid"

type UUIDGenerator struct {
	EnforceLimit bool
	stateHolder  *StateHolder
}

func NewUUIDGenerator(stateHolder *StateHolder) *UUIDGenerator {
	return &UUIDGenerator{
		EnforceLimit: true,
		stateHolder:  stateHolder,
	}
}

// GetUUID reads uint64 byte value for uuid from the state
func (u *UUIDGenerator) GetUUID() (uint64, error) {
	stateBytes, err := u.stateHolder.State().Get("", "", keyUUID, u.EnforceLimit)
	if err != nil {
		return 0, fmt.Errorf("cannot get uuid byte from state: %w", err)
	}
	bytes := slices.EnsureByteSliceSize(stateBytes, 8)

	return binary.BigEndian.Uint64(bytes), nil
}

// SetUUID sets a new uint64 byte value
func (u *UUIDGenerator) SetUUID(uuid uint64) error {
	bytes := make([]byte, 8)
	binary.BigEndian.PutUint64(bytes, uuid)
	err := u.stateHolder.State().Set("", "", keyUUID, bytes, u.EnforceLimit)
	if err != nil {
		return fmt.Errorf("cannot set uuid byte to state: %w", err)
	}
	return nil
}

// GenerateUUID generates a new uuid and persist the data changes into state
func (u *UUIDGenerator) GenerateUUID() (uint64, error) {
	uuid, err := u.GetUUID()
	if err != nil {
		return 0, fmt.Errorf("cannot generate UUID: %w", err)
	}

	err = u.SetUUID(uuid + 1)
	if err != nil {
		return 0, fmt.Errorf("cannot generate UUID: %w", err)
	}
	return uuid, nil
}
