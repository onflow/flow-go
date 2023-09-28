package pebble

import (
	"fmt"

	"github.com/cockroachdb/pebble"
	"github.com/hashicorp/go-multierror"
	"github.com/onflow/flow-go/storage/pebble/registers"
)

func NewBootstrappedRegistersWithPath(dir string) (*Registers, *pebble.DB, error) {
	db, err := initRegisterPebbleDB(dir)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize pebble db: %w", err)
	}
	registers, err := NewRegisters(db)
	if err != nil {
		// fail to initialize regisers (most likely not bootstrapped), close db and return error
		dbClose := db.Close()
		if err != nil {
			err = multierror.Append(err, dbClose)
		}
		return nil, nil, fmt.Errorf("failed to initialize registers: %w", err)
	}
	return registers, db, nil
}

func initRegisterPebbleDB(dir string) (*pebble.DB, error) {
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
