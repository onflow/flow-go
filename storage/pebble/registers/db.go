package registers

import (
	"fmt"

	"github.com/cockroachdb/pebble"

	storagepebble "github.com/onflow/flow-go/storage/pebble"
)

func InitDB(dir string) (*storagepebble.Registers, error) {
	db, err := initPebbleDB(dir)
	if err != nil {
		return nil, err
	}

	return storagepebble.NewRegisters(db)
}

func initPebbleDB(dir string) (*pebble.DB, error) {
	cache := pebble.NewCache(1 << 20)
	// currently pebble is only used for registers
	opts := storagepebble.DefaultPebbleOptions(cache, NewMVCCComparer())
	db, err := pebble.Open(dir, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open db: %w", err)
	}

	return db, nil
}
