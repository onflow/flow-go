package pebble

import (
	"fmt"
	"path"

	"github.com/cockroachdb/pebble"
	"go.uber.org/multierr"

	"github.com/onflow/flow-go/storage/pebble/registers/payload"
)

// library that implements pebble storage for registers
type Registers struct {
	Payloads *payload.Payloads
}

func StoragePath(dir string) string {
	return path.Join(dir, "payload.db")
}

func NewRegisters(dir string, blockCacheSize int64) (*Registers, error) {
	cache := pebble.NewCache(blockCacheSize)
	defer cache.Unref()

	payloadStor, err := payload.NewPayloads(
		StoragePath(dir), cache)
	if err != nil {
		return nil, fmt.Errorf("failed to create payload storage: %w", err)
	}

	return &Registers{
		Payloads: payloadStor,
	}, nil
}

func (l *Registers) Close() (err error) {
	multierr.AppendInto(&err, l.Payloads.Close())
	return
}
