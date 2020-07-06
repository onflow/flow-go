package fvm

import (
	"fmt"

	"github.com/dapperlabs/flow-go/fvm/state"
)

type UUIDGenerator struct {
	uuids *state.UUIDs
}

func NewUUIDGenerator(uuids *state.UUIDs) *UUIDGenerator {
	return &UUIDGenerator{
		uuids: uuids,
	}
}
func (u *UUIDGenerator) GenerateUUID() (uint64, error) {
	uuid, err := u.uuids.GetUUID()
	if err != nil {
		// TODO - Return error once Cadence interface accommodates it
		return 0, fmt.Errorf("cannot get UUID: %w", err)
	}

	defer func() {
		uuid++
		u.uuids.SetUUID(uuid)
	}()

	return uuid, nil
}
