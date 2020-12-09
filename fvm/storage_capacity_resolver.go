package fvm

import (
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

type StorageCapacityResolver func(state.Ledger, flow.Address) (uint64, error)

func CreateResourceStorageCapacityResolver(_ flow.Address) StorageCapacityResolver {
	return func(l state.Ledger, address flow.Address) (uint64, error) {
		return uint64(100000), nil
	}
}
