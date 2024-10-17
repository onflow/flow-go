package storage

import (
	"io"
)

// Iterator is an interface for iterating over key-value pairs in a storage backend.
// A common usage is:
//
//		defer it.Close()
//
//		for it.First(); it.Valid(); it.Next() {
//	 		item := it.IterItem()
//	 	}
type Iterator interface {
	// First seeks to the smallest key greater than or equal to the given key.
	// This method must be called because it's necessary for the badger implementation
	// to move the iteration cursor to the first key in the iteration range.
	// This method must be called before calling Valid, Next, IterItem, or Close.
	// return true if the iterator is pointing to a valid key-value pair after calling First,
	// return false otherwise.
	First() bool

	// Valid returns whether the iterator is positioned at a valid key-value pair.
	// If Valid returns false, the iterator is done and must be closed.
	Valid() bool

	// Next advances the iterator to the next key-value pair.
	// The next key-value pair might be invalid, so you should call Valid() to check.
	Next()

	// IterItem returns the current key-value pair, or nil if Valid returns false.
	// Always to call Valid() before calling IterItem.
	// Note, the returned item is only valid until the Next() method is called.
	IterItem() IterItem

	// Close closes the iterator. Iterator must be closed, otherwise it causes memory leak.
	// No errors expected during normal operation
	Close() error
}

// IterItem is an interface for iterating over key-value pairs in a storage backend.
type IterItem interface {
	// Key returns the key of the current key-value pair
	// Key is only valid until the Iterator.Next() method is called
	// If you need to use it outside its validity, please use KeyCopy
	Key() []byte

	// KeyCopy returns a copy of the key of the item, writing it to dst slice.
	// If nil is passed, or capacity of dst isn't sufficient, a new slice would be allocated and
	// returned.
	KeyCopy(dst []byte) []byte

	// Value returns the value of the current key-value pair
	// The reason it takes a function is to follow badgerDB's API pattern
	// No errors expected during normal operation
	Value(func(val []byte) error) error
}

type IteratorOption struct {
	BadgerIterateKeyOnly bool // default false
}

func DefaultIteratorOptions() IteratorOption {
	return IteratorOption{
		// only needed for badger. ignored by pebble
		BadgerIterateKeyOnly: false,
	}
}

type Reader interface {
	// Get gets the value for the given key. It returns ErrNotFound if the DB
	// does not contain the key.
	// other errors are exceptions
	//
	// The caller should not modify the contents of the returned slice, but it is
	// safe to modify the contents of the `key` argument after Get returns. The
	// returned slice will remain valid until the returned Closer is closed.
	// when err == nil, the caller MUST call closer.Close() or a memory leak will occur.
	Get(key []byte) (value []byte, closer io.Closer, err error)

	// NewIter returns a new Iterator for the given key prefix range [startPrefix, endPrefix], both inclusive.
	// We require that startPrefix ≤ endPrefix (otherwise this function errors).
	// Specifically, all keys that meet ANY of the following conditions are included in the iteration:
	//   - have a prefix equal to startPrefix OR
	//   - have a prefix equal to the endPrefix OR
	//   - have a prefix that is lexicographically between startPrefix and endPrefix
	NewIter(startPrefix, endPrefix []byte, ops IteratorOption) (Iterator, error)
}

// Writer is an interface for batch writing to a storage backend.
// One Writer instance cannot be used concurrently by multiple goroutines.
type Writer interface {
	// Set sets the value for the given key. It overwrites any previous value
	// for that key; a DB is not a multi-map.
	//
	// It is safe to modify the contents of the arguments after Set returns.
	// No errors expected during normal operation
	Set(k, v []byte) error

	// Delete deletes the value for the given key. Deletes are blind all will
	// succeed even if the given key does not exist.
	//
	// It is safe to modify the contents of the arguments after Delete returns.
	// No errors expected during normal operation
	Delete(key []byte) error

	// DeleteByRange removes all keys with a prefix that falls within the
	// range [start, end], both inclusive.
	// No errors expected during normal operation
	DeleteByRange(globalReader Reader, startPrefix, endPrefix []byte) error
}

// ReaderBatchWriter is an interface for reading and writing to a storage backend.
// It is useful for performing a related sequence of reads and writes, after which you would like
// to modify some non-database state if the sequence completed successfully (via AddCallback).
// If you are not using AddCallback, avoid using ReaderBatchWriter: use Reader and Writer directly.
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

// DB is an interface for a database store that provides a reader and a writer.
type DB interface {
	// Reader returns a database-backed reader which reads the latest
	// committed global database state
	Reader() Reader

	// WithReaderBatchWriter creates a batch writer and allows the caller to perform
	// atomic batch updates to the database.
	// Any error returned are considered fatal and the batch is not committed.
	WithReaderBatchWriter(func(ReaderBatchWriter) error) error
}

// OnlyWriter is an adapter to convert a function that takes a Writer
// to a function that takes a ReaderBatchWriter.
func OnlyWriter(fn func(Writer) error) func(ReaderBatchWriter) error {
	return func(rw ReaderBatchWriter) error {
		return fn(rw.Writer())
	}
}

// OnCommitSucceed adds a callback to execute after the batch has been successfully committed.
func OnCommitSucceed(b ReaderBatchWriter, onSuccessFn func()) {
	b.AddCallback(func(err error) {
		if err == nil {
			onSuccessFn()
		}
	})
}

// StartEndPrefixToLowerUpperBound returns the lower and upper bounds for a range of keys
// specified by the start and end prefixes.
// the lower and upper bounds are used for the key iteration.
// The return value lowerBound specifies the smallest key to iterate and it's inclusive.
// The return value upperBound specifies the largest key to iterate and it's exclusive (not inclusive)
// The return value hasUpperBound specifies whether there is upperBound
// in order to match all keys prefixed with `endPrefix`, we increment the bytes of `endPrefix` by 1,
// for instance, to iterate keys between "hello" and "world",
// we use "hello" as LowerBound, "worle" as UpperBound, so that "world", "world1", "worldffff...ffff"
// will all be included.
// In the case that the endPrefix is all 1s, such as []byte{0xff, 0xff, ...}, there is no upper-bound,
// it returns (startPrefix, nil, false)
func StartEndPrefixToLowerUpperBound(startPrefix, endPrefix []byte) (lowerBound, upperBound []byte, hasUpperBound bool) {
	// if the endPrefix is all 1s, such as []byte{0xff, 0xff, ...}, there is no upper-bound
	// so we return the startPrefix as the lower-bound, and nil as the upper-bound, and false for hasUpperBound
	upperBound = PrefixUpperBound(endPrefix)
	if upperBound == nil {
		return startPrefix, nil, false
	}

	return startPrefix, upperBound, true
}

// PrefixUpperBound returns a key K such that all possible keys beginning with the input prefix
// sort lower than K according to the byte-wise lexicographic key ordering.
// This is used to define an upper bound for iteration, when we want to iterate over
// all keys beginning with a given prefix.
// referred to https://pkg.go.dev/github.com/cockroachdb/pebble#example-Iterator-PrefixIteration
// when the prefix is all 1s, such as []byte{0xff}, or []byte(0xff, 0xff} etc, there is no upper-bound
// It returns nil in this case.
func PrefixUpperBound(prefix []byte) []byte {
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
