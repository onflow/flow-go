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

// SeekLE (seek less than or equal) returns given key if present.  Otherwise,
// it returns the largest key that is less than the given key within startPrefix
// and endPrefix.  Keys are ordered in lexicographical order.
// This function returns error if given key is outside range of startPrefix and endPrefix.
func (i *pebbleSeeker) SeekLE(startPrefix, endPrefix []byte, key []byte) (_ []byte, _ bool, errToReturn error) {
	lowerBound, upperBound, hasUpperBound := storage.StartEndPrefixToLowerUpperBound(startPrefix, endPrefix)

	if bytes.Compare(key, startPrefix) < 0 {
		return nil, false, errors.New("key must be greater than or equal to startPrefix key")
	}

	if hasUpperBound && bytes.Compare(key, endPrefix) > 0 {
		return nil, false, errors.New("key must be less than or equal to endPrefix key")
	}

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
		return nil, false, fmt.Errorf("can not create iterator: %w", err)
	}
	defer func() {
		errToReturn = merr.CloseAndMergeError(iter, errToReturn)
	}()

	// Seek given key if present.

	valid := iter.SeekGE(key)
	if valid {
		if bytes.Equal(iter.Key(), key) {
			return slices.Clone(iter.Key()), true, nil
		}
	}

	// Seek smallest key less than the given key.

	valid = iter.SeekLT(key)
	if !valid {
		return nil, false, nil
	}

	return slices.Clone(iter.Key()), true, nil
}
