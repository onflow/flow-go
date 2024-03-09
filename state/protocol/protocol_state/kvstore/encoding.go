package kvstore

import (
	"bytes"

	"github.com/vmihailenco/msgpack/v4"

	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol/protocol_state"
)

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
// Errors:
//   - ErrUnsupportedVersion if input version is not supported
func VersionedDecode(version uint64, bz []byte) (protocol_state.API, error) {
	var target protocol_state.API
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
