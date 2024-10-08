package storage

import (
	"io"
)

// Iterator is an interface for iterating over key-value pairs in a storage backend.
type Iterator interface {
	// SeekGE seeks to the smallest key greater than or equal to the given key.
	SeekGE()

	// Valid returns whether the iterator is positioned at a valid key-value pair.
	Valid() bool

	// Next advances the iterator to the next key-value pair.
	Next()

	// Key returns the key of the current key-value pair, or nil if done.
	IterItem() IterItem

	// Close closes the iterator. Iterator must be closed, otherwise it causes memory leak.
	Close() error
}

// IterItem is an interface for iterating over key-value pairs in a storage backend.
type IterItem interface {
	Key() []byte

	// Value returns the value of the current key-value pair
	// The reason it takes a function is to follow badgerDB's API pattern
	Value(func(val []byte) error) error
}

type IteratorOption struct {
	IterateKeyOnly bool // default false
}

func DefaultIteratorOptions() IteratorOption {
	return IteratorOption{
		IterateKeyOnly: false, // only needed for badger. ignored by pebble
	}
}

type Reader interface {
	// Get gets the value for the given key. It returns ErrNotFound if the DB
	// does not contain the key.
	//
	// The caller should not modify the contents of the returned slice, but it is
	// safe to modify the contents of the argument after Get returns. The
	// returned slice will remain valid until the returned Closer is closed. On
	// success, the caller MUST call closer.Close() or a memory leak will occur.
	Get(key []byte) (value []byte, closer io.Closer, err error)

	// NewIter returns a new Iterator for the given key range [startPrefix, endPrefix], both inclusive.
	NewIter(startPrefix, endPrefix []byte, ops IteratorOption) (Iterator, error)
}

// Writer is an interface for batch writing to a storage backend.
type Writer interface {
	// Set sets the value for the given key. It overwrites any previous value
	// for that key; a DB is not a multi-map.
	//
	// It is safe to modify the contents of the arguments after Set returns.
	Set(k, v []byte) error

	// Delete deletes the value for the given key. Deletes are blind all will
	// succeed even if the given key does not exist.
	//
	// It is safe to modify the contents of the arguments after Delete returns.
	Delete(key []byte) error

	// DeleteByRange removes all keys with a prefix that falls within the
	// range [start, end], both inclusive.
	DeleteByRange(globalReader Reader, startPrefix, endPrefix []byte) error
}

// ReaderBatchWriter is an interface for reading and writing to a storage backend.
type ReaderBatchWriter interface {
	// GlobalReader returns a database-backed reader which reads the latest committed global database state ("read-committed isolation").
	// This reader will not read writes written to ReaderBatchWriter.Writer until the write batch is committed.
	// This reader may observe different values for the same key on subsequent reads.
	GlobalReader() Reader

	// Writer returns a writer associated with a batch of writes. The batch is pending until it is committed.
	// When we `Write` into the batch, that write operation is added to the pending batch, but not committed.
	// The commit operation is atomic w.r.t. the batch; either all writes are applied to the database, or no writes are.
	// Note:
	// - The writer cannot be used concurrently for writing.
	Writer() Writer

	// AddCallback adds a callback to execute after the batch has been flush
	// regardless the batch update is succeeded or failed.
	// The error parameter is the error returned by the batch update.
	AddCallback(func(error))
}

// OnCommitSucceed adds a callback to execute after the batch has been successfully committed.
func OnCommitSucceed(b ReaderBatchWriter, onSuccessFn func()) {
	b.AddCallback(func(err error) {
		if err == nil {
			onSuccessFn()
		}
	})
}

func StartEndPrefixToLowerUpperBound(startPrefix, endPrefix []byte) (lowerBound, upperBound []byte) {
	// LowerBound specifies the smallest key to iterate and it's inclusive.
	// UpperBound specifies the largest key to iterate and it's exclusive (not inclusive)
	// in order to match all keys prefixed with the `end` bytes, we increment the bytes of end by 1,
	// for instance, to iterate keys between "hello" and "world",
	// we use "hello" as LowerBound, "worle" as UpperBound, so that "world", "world1", "worldffff...ffff"
	// will all be included.
	return startPrefix, prefixUpperBound(endPrefix)
}

// prefixUpperBound returns a key K such that all possible keys beginning with the input prefix
// sort lower than K according to the byte-wise lexicographic key ordering used by Pebble.
// This is used to define an upper bound for iteration, when we want to iterate over
// all keys beginning with a given prefix.
// referred to https://pkg.go.dev/github.com/cockroachdb/pebble#example-Iterator-PrefixIteration
func prefixUpperBound(prefix []byte) []byte {
	end := make([]byte, len(prefix))
	copy(end, prefix)
	for i := len(end) - 1; i >= 0; i-- {
		// increment the bytes by 1
		end[i] = end[i] + 1
		if end[i] != 0 {
			return end[:i+1]
		}
	}
	return nil // no upper-bound
}
