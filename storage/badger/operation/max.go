package operation

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v2"
)

// maxKey is the biggest allowed key size in badger
const maxKey = 65000

// max holds the maximum length of keys in the database; in order to optimize
// the end prefix of iteration, we need to know how many `0xff` bytes to add.
var max uint32

// we initialize max to maximum size, to detect if it wasn't set yet
func init() {
	max = maxKey
}

// InitMax retrieves the maximum key length to have it interally in the
// package after restarting.
func InitMax(tx *badger.Txn) error {
	key := makePrefix(codeMax)
	item, err := tx.Get(key)
	if errors.Is(err, badger.ErrKeyNotFound) { // just keep zero value as default
		max = 0
		return SetMax(tx)
	}
	if err != nil {
		return fmt.Errorf("could not get max: %w", err)
	}
	_ = item.Value(func(val []byte) error {
		max = binary.LittleEndian.Uint32(val)
		return nil
	})
	return nil
}

// SetMax sets the value for the maximum key length used for efficient iteration.
func SetMax(tx *badger.Txn) error {
	key := makePrefix(codeMax)
	val := make([]byte, 4)
	binary.LittleEndian.PutUint32(val, max)
	err := tx.Set(key, val)
	if err != nil {
		return fmt.Errorf("could not set max: %w", err)
	}
	return nil
}
