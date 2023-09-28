package pebble

import (
	"fmt"

	"github.com/cockroachdb/pebble"
	"github.com/onflow/flow-go/storage/pebble/registers"
)

// NewBootstrappedRegistersWithPath initializes a new Registers instance with a pebble db
// if the database is not initialized, it close the database and return pebble.ErrNotBootstrapped
func NewBootstrappedRegistersWithPath(dir string) (*Registers, *pebble.DB, error) {
	db, err := openRegisterPebbleDB(dir)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize pebble db: %w", err)
	}
	registers, err := NewRegisters(db)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize registers: %w", err)
	}
	return registers, db, nil
}

// openRegisterPebbleDB opens the database
func openRegisterPebbleDB(dir string) (*pebble.DB, error) {
	cache := pebble.NewCache(1 << 20)
	defer cache.Unref()
	// currently pebble is only used for registers
	opts := DefaultPebbleOptions(cache, registers.NewMVCCComparer())
	db, err := pebble.Open(dir, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open db: %w", err)
	}

	return db, nil
}
