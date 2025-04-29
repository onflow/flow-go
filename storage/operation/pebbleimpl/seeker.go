package pebbleimpl

import (
	"bytes"
	"errors"
	"fmt"
	"slices"

	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/merr"

	"github.com/cockroachdb/pebble"
)

type pebbleSeeker struct {
	reader pebble.Reader
}

var _ storage.Seeker = (*pebbleSeeker)(nil)

func newPebbleSeeker(reader pebble.Reader) *pebbleSeeker {
	return &pebbleSeeker{
		reader: reader,
	}
}

// SeekLE (seek less than or equal) returns the largest key in lexicographical
// order within inclusive range of [startPrefix, key].
// This function returns an error if specified key is less than startPrefix.
// This function returns storage.ErrNotFound if a key that matches
// the specified criteria is not found.
func (i *pebbleSeeker) SeekLE(startPrefix, key []byte) (_ []byte, errToReturn error) {

	if bytes.Compare(key, startPrefix) < 0 {
		return nil, errors.New("key must be greater than or equal to startPrefix key")
	}

	lowerBound, upperBound, hasUpperBound := storage.StartEndPrefixToLowerUpperBound(startPrefix, key)

	options := pebble.IterOptions{
		LowerBound: lowerBound,
		UpperBound: upperBound,
	}

	// Setting UpperBound to nil if there is no upper bound
	if !hasUpperBound {
		options.UpperBound = nil
	}

	iter, err := i.reader.NewIter(&options)
	if err != nil {
		return nil, fmt.Errorf("can not create iterator: %w", err)
	}
	defer func() {
		errToReturn = merr.CloseAndMergeError(iter, errToReturn)
	}()

	// Seek given key if present.

	valid := iter.SeekGE(key)
	if valid {
		if bytes.Equal(iter.Key(), key) {
			return slices.Clone(key), nil
		}
	}

	// Seek largest key less than the given key.

	valid = iter.SeekLT(key)
	if !valid {
		return nil, storage.ErrNotFound
	}

	return slices.Clone(iter.Key()), nil
}
