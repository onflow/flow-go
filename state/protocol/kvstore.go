package protocol

import "github.com/onflow/flow-go/model/flow"

// This file contains versioned read interface to the Protocol State's
// key-value store and are used by the Protocol State Machine.
//
// When a key is added or removed, this requires a new protocol state version:
//  - Create a new versioned model in ./protocol_state/kvstore/models.go (eg. modelv3 if latest model is modelv2)
//  - Update the KVStoreReader and protocol_state.KVStoreAPI interfaces to include any new keys

// KVStoreReader is the latest read-only interface to the Protocol State key-value store
// at a particular block.
//
// Caution:
// Engineers evolving this interface must ensure that it is backwards-compatible
// with all versions of Protocol State Snapshots that can be retrieved from the local
// database, which should exactly correspond to the versioned model types defined in
// ./kvstore/models.go
type KVStoreReader interface {
	// ID returns an identifier for this key-value store snapshot by hashing internal fields.
	// Two different model versions containing the same data must have different IDs.
	// New models should use `makeVersionedModelID` to implement ID.
	ID() flow.Identifier

	// v0/v1

	VersionedEncodable

	// GetProtocolStateVersion returns the Protocol State Version that created the specific
	// Snapshot backing this interface instance. Slightly simplified, the Protocol State
	// Version defines the key-value store's data model (specifically, the set of all keys
	// and the respective type for each corresponding value).
	// Generally, changes in the protocol state version correspond to changes in the set
	// of key-value pairs which are supported, and which model is used for serialization.
	// The protocol state version is updated by UpdateKVStoreVersion service events.
	GetProtocolStateVersion() uint64

	// GetVersionUpgrade returns the upgrade version of protocol.
	// VersionUpgrade is a view-based activator that specifies the version which has to be applied
	// and the view from which on it has to be applied. It may return the current protocol version
	// with a past view if the upgrade has already been activated.
	GetVersionUpgrade() *ViewBasedActivator[uint64]

	// GetEpochStateID returns the state ID of the epoch state.
	// This is part of the most basic model and is used to commit the epoch state to the KV store.
	GetEpochStateID() flow.Identifier

	// GetEpochExtensionViewCount returns the number of views for a hypothetical epoch extension. Note
	// that this value can change at runtime (through a service event). When a new extension is added,
	// the view count is used right at this point in the protocol state's evolution. In other words,
	// different extensions can have different view counts.
	GetEpochExtensionViewCount() uint64

	// GetFinalizationSafetyThreshold [t] defines a deadline for sealing the EpochCommit
	// service event near the end of each epoch - the "epoch commitment deadline".
	// Given a safety threshold t, the deadline for an epoch with final view f is:
	//   Epoch Commitment Deadline: d=f-t
	//
	//                     Epoch Commitment Deadline
	//   EPOCH N           ↓                 EPOCH N+1
	//   ...---------------|---------------| |-----...
	//   view:             d<·····t·······>f
	//
	// DEFINITION:
	// This deadline is used to determine when to trigger epoch emergency fallback mode.
	// Epoch Emergency Fallback mode is triggered when the EpochCommit service event
	// fails to be sealed.
	//
	// Example: A service event is emitted in block A. The seal for A is included in C.
	// A<-B(RA)<-C(SA)<-...<-R
	//
	// A service event S is considered sealed w.r.t. a reference block R if:
	// * S was emitted during execution of some block A, s.t. A is an ancestor of R
	// * The seal for block A was included in some block C, s.t C is an ancestor of R
	//
	// When we finalize the first block B with B.View >= d:
	//  - HAPPY PATH: If an EpochCommit service event has been sealed w.r.t. B, no action is taken.
	//  - FALLBACK PATH: If no EpochCommit service event has been sealed w.r.t. B,
	//    Epoch Fallback Mode [EFM] is triggered.
	//
	// CONTEXT:
	// The epoch commitment deadline exists to ensure that all nodes agree on
	// whether Epoch Fallback Mode is triggered for a particular epoch, before
	// the epoch actually ends. In particular, all nodes will agree about EFM
	// being triggered (or not) if at least one block with view in [d, f] is
	// finalized - in other words, we require at least one block being finalized
	// after the epoch commitment deadline, and before the next epoch begins.
	//
	// It should be noted that we are employing a heuristic here, which succeeds with
	// overwhelming probability of nearly 1. However, theoretically it is possible that
	// no blocks are finalized within t views. In this edge case, the nodes would have not
	// detected the epoch commit phase failing and the protocol would just halt at the end
	// of the epoch. However, we emphasize that this is extremely unlikely, because the
	// probability of randomly selecting t faulty leaders in sequence decays to zero
	// exponentially with increasing t. Furthermore, failing to finalize blocks for a
	// noticeable period entails halting block sealing, which would trigger human
	// intervention on much smaller time scales than t views.
	// Therefore, t should be chosen such that it takes more than 30mins to pass t views
	// under happy path operation. Significant larger values are ok, but t views equalling
	// 30 mins should be seen as a lower bound.
	//
	// When selecting a threshold value, ensure:
	//  * The deadline is after the end of the DKG, with enough buffer between
	//    the two that the EpochCommit event is overwhelmingly likely to be emitted
	//    before the deadline, if it is emitted at all.
	//  * The buffer between the deadline and the final view of the epoch is large
	//    enough that the network is overwhelming likely to finalize at least one
	//    block with a view in this range
	//
	GetFinalizationSafetyThreshold() uint64
}

// VersionedEncodable defines the interface for a versioned key-value store independent
// of the set of keys which are supported. This allows the storage layer to support
// storing different key-value model versions within the same software version.
type VersionedEncodable interface {
	// VersionedEncode encodes the key-value store, returning the version separately
	// from the encoded bytes.
	// No errors are expected during normal operation.
	VersionedEncode() (uint64, []byte, error)
}

// ViewBasedActivator allows setting value that will be active from specific view.
type ViewBasedActivator[T any] struct {
	Data           T
	ActivationView uint64 // first view at which Data will take effect
}
