package fvm

import (
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/storage"
)

// TODO(patrick): rm once flow-emulator is updated
type Blocks = environment.Blocks

// TODO(patrick): rm once flow-emulator is updated
func NewBlockFinder(storage storage.Headers) Blocks {
	return environment.NewBlockFinder(storage)
}
