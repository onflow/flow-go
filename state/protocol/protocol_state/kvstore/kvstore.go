package kvstore

import "errors"

var ErrNotExists = errors.New("kvstore: no value set for key")

// GenericKVModel implements a fixed model for keys that can be stored in
// the store. Conceptually it is a container around the KeyValuePairs with
// version information. While the `GenericKVModel` container is only allowed
// to change during sporks, the payload represented by `KeyValuePairs` can be
// updated repeatedly only requiring rolling software upgrades that understand
// the updated format but without requiring a spork.

// KVModel represents the latest key value store model the current node software is able to read.
type KVModel = KVModelV1

// KVModelV1 is v1 of the Protocol State key value store.
type KVModelV1 struct {
	// KVStoreVersion is the version of the Protocol State key value store.
	KVStoreVersion uint64
}

type KVReaderV1 interface {
	GetKVStoreVersion() (uint64, error)
}

type KVWriterV1 interface {
	SetKVStoreVersion(uint64) error
}

type KVInterfaceV1 interface {
	GetKVStoreVersion() (uint64, error)
	SetKVStoreVersion(uint64) error
}

type KVInterface KVInterfaceV1
