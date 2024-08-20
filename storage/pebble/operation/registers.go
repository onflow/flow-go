package operation

import (
	"encoding/binary"

	"github.com/cockroachdb/pebble"
)

func RetrieveHeight(db *pebble.DB, key []byte) (uint64, error) {
	res, closer, err := db.Get(key)
	if err != nil {
		return 0, convertNotFoundError(err)
	}
	defer closer.Close()

	return binary.BigEndian.Uint64(res), nil
}
