package cmd

import (
	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/storage"
)

// WithStorage runs the given function with the storage dependending on the flags
// only one flag (datadir / pebble-dir) is allowed to be set
func WithStorage(f func(storage.DB) error) error {
	flagDBs := common.ReadDBFlags()
	return common.WithStorage(flagDBs, f)
}
