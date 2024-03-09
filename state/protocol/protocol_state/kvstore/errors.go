package kvstore

import "errors"

// ErrKeyNotSet is a sentinel returned when a key is queried and no value has been set.
// The key must exist in the currently active key-value store version. This sentinel
// is used to communicate an empty/unset value rather than using zero or nil values.
// This sentinel is applicable on a key-by-key basis: some keys will always have a value
// set, others will support unset values.
var ErrKeyNotSet = errors.New("kvstore: no value set for key")

// ErrKeyNotSupported is a sentinel returned when a key is read or written, but
// the key does not exist in the currently active version of the key-value store.
// This can happen in two circumstances, for example:
//  1. Current model is v2, software supports v3, and we query a key which was newly added in v3.
//  2. Current model is v3 and we query a key which was added in v2 then removed  in v3
var ErrKeyNotSupported = errors.New("kvstore: key is not supported in current store version")

// ErrUnsupportedVersion is a sentinel returned when we attempt to decode a key-value
// store instance, but provide an unsupported version. This could happen if we accept
// an already-encoded key-value store instance from an external source (should be
// avoided in general) or if the node software version is downgraded.
var ErrUnsupportedVersion = errors.New("kvstore: unsupported version")

// ErrInvalidUpgradeVersion is a sentinel returned when we attempt to set a new kvstore version
// via a ProtocolStateVersionUpgrade event, but the new version is not strictly greater than
// the current version. This error happens when smart contract has different understanding of
// the protocol state version than the node software.
var ErrInvalidUpgradeVersion = errors.New("kvstore: invalid upgrade version")

// ErrInvalidActivationView is a sentinel returned when we attempt to process a KV store update
// which has an activation view that is not strictly greater than the current view.
var ErrInvalidActivationView = errors.New("kvstore: invalid activation view")
