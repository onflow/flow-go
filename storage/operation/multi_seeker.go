package operation

import (
	"bytes"
	"errors"

	"github.com/onflow/flow-go/storage"
)

type multiSeeker struct {
	seekers []storage.Seeker
}

var _ storage.Seeker = (*multiSeeker)(nil)

// NewMultiSeeker returns a Seeker that consists of multiple seekers
// in the provided order.
func NewMultiSeeker(seekers ...storage.Seeker) storage.Seeker {
	if len(seekers) == 1 {
		return seekers[0]
	}
	return &multiSeeker{seekers: seekers}
}

// SeekLE (seek less than or equal) returns the largest key in lexicographical
// order within inclusive range of [startPrefix, key].
// This function returns an error if specified key is less than startPrefix.
// This function returns storage.ErrNotFound if a key that matches
// the specified criteria is not found.
func (b *multiSeeker) SeekLE(startPrefix, key []byte) ([]byte, error) {
	if bytes.Compare(key, startPrefix) < 0 {
		return nil, errors.New("key must be greater than or equal to startPrefix key")
	}

	// Seek less than or equal from the last seeker first.
	for i := len(b.seekers) - 1; i >= 0; i-- {
		seeker := b.seekers[i]
		key, err := seeker.SeekLE(startPrefix, key)
		if err == nil || !errors.Is(err, storage.ErrNotFound) {
			return key, err
		}
	}

	return nil, storage.ErrNotFound
}
