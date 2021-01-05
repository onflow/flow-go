package service

import (
	"fmt"

	"github.com/onflow/flow-go/fvm/state"
)

type StatefulUUIDGenerator struct {
	uuids *state.UUIDs
}

func NewStatefulUUIDGenerator(uuids *state.UUIDs) *StatefulUUIDGenerator {
	return &StatefulUUIDGenerator{
		uuids: uuids,
	}
}
func (u *StatefulUUIDGenerator) GenerateUUID() (uint64, error) {
	uuid, err := u.uuids.GetUUID()
	if err != nil {
		return 0, fmt.Errorf("cannot get UUID: %w", err)
	}

	defer func() {
		uuid++
		u.uuids.SetUUID(uuid)
	}()

	return uuid, nil
}
