package fvm

import (
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/storage"
)

// TODO(patrick): rm after https://github.com/onflow/flow-emulator/pull/229
// is merged and integrated.
type Blocks = environment.Blocks

// TODO(patrick): rm after https://github.com/onflow/flow-emulator/pull/229
// is merged and integrated.
func NewBlockFinder(storage storage.Headers) Blocks {
	return environment.NewBlockFinder(storage)
}
