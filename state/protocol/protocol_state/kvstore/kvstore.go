package kvstore

import (
	"errors"
)

// ErrKeyNotSet is a sentinel returned when a key is queried and no value has been set.
//   - key does not exist in current kv model version
//   - key does exist model, but has not been set
var ErrKeyNotSet = errors.New("kvstore: no value set for key")

// GenericKVModel implements a fixed model for keys that can be stored in
// the store. Conceptually it is a container around the KeyValuePairs with
// version information. While the `GenericKVModel` container is only allowed
// to change during sporks, the payload represented by `KeyValuePairs` can be
// updated repeatedly only requiring rolling software upgrades that understand
// the updated format but without requiring a spork.

// TODOs:
// - [x] add additional version (unused v0)
// - [ ] add encode/decode logic
// - [ ] unit tests (encode decode)

// TODO pretty sure we can remove this and use versionedSerializer for storage layer and latestApi interface for pstate interaction
type GenericKVModel[KVPairs any] struct {
	// Version specifies the current version of the key-value store.
	// It can be updated by an UpdateKVStoreVersion service event.
	// TODO: confirm that this version represents ProtocolStateVersion (ie. there is no separate KVStoreVersion)
	//  and update this godoc
	Version uint64
	// Store is the actual key-value store with a fixed set of keys
	Store KVPairs
}

// LatestKVModel represents the latest key value store model the current node software is able to read.
// By convention, the node software must also be able to read all previous model versions.
type LatestKVModel = GenericKVModel[KVPairsV1]

// KV PAIRS
// These concrete types define the structure of the underlying key-value store,
// essentially enumerating the set of keys and values that are supported.

// TODO should versioned KVPairs structures be private?

// KVPairsV0 is v0 of the Protocol State key value store.
// This model version is not intended to ever be the latest version supported by
// any software version. Since it is important that the store support managing
// different model version, this is here so that we can test the implementation
// with multiple supported KV model versions from the beginning.
type KVPairsV0 struct{}

var _ KVApiV0 = new(KVPairsV0)

func (pairs *KVPairsV0) VersionedEncode() (uint64, []byte, error) {
	return versionedEncode(0, pairs)
}

func (pairs *KVPairsV0) GetInvalidEpochTransitionAttempted() (bool, error) {
	return false, ErrKeyNotSet
}

// KVPairsV1 is v1 of the Protocol State key value store.
type KVPairsV1 struct {
	// InvalidEpochTransitionAttempted encodes whether an invalid epoch transition
	// has been detected in this fork. Under normal operations, this value is false.
	// Node-internally, the EpochFallback notification is emitted when a block is
	// finalized that changes this flag from false to true.
	//
	// Currently, the only possible state transition is false → true.
	//
	// TODO: I've added this here so that we have at least 2 model versions to begin,
	//  which simplifies testing. We can choose to add different/more KV pairs to v1.
	InvalidEpochTransitionAttempted bool
}

func (pairs *KVPairsV1) VersionedEncode() (uint64, []byte, error) {
	return versionedEncode(1, pairs)
}

func (pairs *KVPairsV1) GetInvalidEpochTransitionAttempted() (bool, error) {
	return pairs.InvalidEpochTransitionAttempted, nil
}

// VersionedEncodable defines the interface for a versioned key-value store,
// independently of the set of keys which are supported.
// The storage layer will:
//   - when storing, having a KVApi instance implementing this interface, then store
//     a structure {version, encodedKVStore} without needing to know kv pairs
//   - when retrieving, retrieve {version, encodedKVStore} and use kvstore.Decode([]byte) (latestApi, error)
type VersionedEncodable interface {
	VersionedEncode() (uint64, []byte, error)
}

// KV READERS
// These interfaces provide read-only access to the underlying key-value store
// and are used to provide access to data in the key-value store to other components.

// TODO: should the versioned interfaces be private?

// TODO: should ProtocolStateVersion/KVStoreVersion go in here?
type KVReaderV0 interface {
}

type KVReaderV1 interface {
	KVReaderV0
	GetInvalidEpochTransitionAttempted() (bool, error)
}

// KV APIS
// These interfaces provide read/write access to the underlying key-value store
// and are used by the Protocol State Machine.

// TODO: should the versioned interfaces be private?

type KVApiV0 interface {
	VersionedEncodable
	KVReaderV0
}

type KVApiV1 interface {
	KVApiV0
	KVReaderV1
	SetInvalidEpochTransitionAttempted(attempted bool) error
}

type LatestKVApi = KVApiV1
