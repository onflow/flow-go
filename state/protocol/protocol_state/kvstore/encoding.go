package kvstore

import (
	"bytes"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/vmihailenco/msgpack/v4"
)

// VersionedEncodable defines the interface for a versioned key-value store independent
// of the set of keys which are supported. This allows the storage layer to support
// storing different key-value model versions within the same software version.
type VersionedEncodable interface {
	// VersionedEncode encodes the key-value store, returning the version separately
	// from the encoded bytes.
	// No errors are expected during normal operation.
	VersionedEncode() (uint64, []byte, error)
}

// versionedEncode is a helper function for implementing VersionedEncodable.
// No errors are expected during normal operation.
func versionedEncode(version uint64, pairs any) (uint64, []byte, error) {
	bz, err := msgpack.Marshal(pairs)
	if err != nil {
		return 0, nil, irrecoverable.NewExceptionf("could not encode kvstore (version=%d): %w", version, err)
	}
	return version, bz, nil
}

// VersionedDecode decodes a serialized key-value store instance with the given version.
// No errors are expected during normal operation.
func VersionedDecode(version uint64, bz []byte) (API, error) {
	var target API
	switch version {
	case 0:
		target = new(modelv0)
	case 1:
		target = new(modelv1)
	default:
		return nil, ErrUnsupportedVersion
	}
	err := msgpack.NewDecoder(bytes.NewBuffer(bz)).Decode(&target)
	if err != nil {
		return nil, irrecoverable.NewExceptionf("could not decode kvstore (version=%d): %w", version, err)
	}
	return target, nil
}
