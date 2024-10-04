package storage

import (
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

// RegisterID creates a RegisterID from owner and key
func RegisterID(owner []byte, key []byte) flow.RegisterID {
	return flow.NewRegisterID(flow.BytesToAddress(owner), string(key))
}

var EmptySnapshot = &emptySnapshot{}

type emptySnapshot struct{}

var _ types.BackendStorageSnapshot = &emptySnapshot{}

func (s *emptySnapshot) GetValue(owner, key []byte) ([]byte, error) {
	return nil, nil
}
