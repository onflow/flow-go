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

	// GetFinalizationSafetyThreshold returns the FinalizationSafetyThreshold's current value `t`.
	// The FinalizationSafetyThreshold is maintained by the protocol state, with correctness and
	// consistency of updates across all nodes guaranteed by BFT consensus.
	//
	// In a nutshell, the FinalizationSafetyThreshold is a protocol axiom:
	// It specifies the number of views `t`, such that when an honest node enters or surpasses
	// view `v+t` the latest finalized view must be larger or equal to `v`. The value `t` is an
	// empirical threshold, which must be chosen large enough that the probability of finalization
	// halting for `t` or more views vanishes in practise. In the unlikely scenario that this
	// threshold is exceeded, the protocol should halt.
	// Formally, HotStuff (incl. its Jolteon derivative) provides no guarantees that finalization
	// proceeds within `t` views, for _any_ value of `t`. Therefore, the FinalizationSafetyThreshold
	// is an additional limitation on the *liveness* guarantees that HotStuff (Jolteon) provides.
	// When entering view `v+t`, *safety-relevant* protocol logic should *confirm* that finalization
	// has reached or exceeded view `v`.
	//
	// EXAMPLE:
	// Given a threshold value `t`, the deadline for an epoch with final view `f` is:
	//   Epoch Commitment Deadline: d=f-t
	//
	//                     Epoch Commitment Deadline
	//   EPOCH N           ↓                            EPOCH N+1
	//   ...---------------|--------------------------| |-----...
	//                     ↑                          ↑ ↑
	//   view:             d············t············>⋮ f+1
	//
	// This deadline is used to determine when to trigger Epoch Fallback Mode [EFM]:
	// if no valid configuration for epoch N+1 has been determined by view `d`, the
	// protocol enters EFM for the following reason:
	//  * By the time a node surpasses the last view `f` of epoch N, it must know the leaders
	//    for every view of epoch N+1.
	//  * The leader selection for epoch N+1 is only unambiguously determined, if the configuration
	//    for epoch N+1 has been finalized. (Otherwise, different forks could contain different
	//    consensus committees for epoch N+1, which would lead to different leaders. Only finalization
	//    resolves this ambiguity by finalizing one and orphaning epoch configurations possibly
	//    contained in competing forks).
	//  * The latest point where we could still finalize a configuration for Epoch N+1 is the last view
	//    `f` of epoch N. As finalization is permitted to take up to `t` views, a valid configuration
	//    for epoch N+1 must be available at latest by view d=f-t.
	//
	// When selecting a threshold value, ensure:
	//  * The deadline is after the end of the DKG, with enough buffer between
	//    the two that the EpochCommit event is overwhelmingly likely to be emitted
	//    before the deadline, if it is emitted at all.
	//  * The buffer between the deadline and the final view of the epoch is large
	//    enough that the network is overwhelming likely to finalize at least one
	//    block with a view in this range
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
