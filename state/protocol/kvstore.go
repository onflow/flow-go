package protocol

import (
	"io"
	"math"
	"slices"

	"github.com/ethereum/go-ethereum/rlp"

	"github.com/onflow/flow-go/model/flow"
)

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
	// and the view from which on it has to be applied. After an upgrade activation view has passed,
	// the (version, view) data remains in the state until the next upgrade is scheduled (essentially
	// persisting the most recent past update until a subsequent update is scheduled).
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

	// v2

	// GetCadenceComponentVersion returns the Cadence component version.
	// Error Returns:
	//   - kvstore.ErrKeyNotSupported if invoked on a KVStore instance before v2.
	GetCadenceComponentVersion() (MagnitudeVersion, error)
	// GetCadenceComponentVersionUpgrade returns the most recent upgrade for the Cadence Component Version,
	// if one exists (otherwise returns nil). The upgrade will be returned even if it has already been applied.
	// Returns nil if invoked on a KVStore instance before v2.
	GetCadenceComponentVersionUpgrade() *ViewBasedActivator[MagnitudeVersion]

	// GetExecutionComponentVersion returns the Execution component version.
	// Error Returns:
	//   - kvstore.ErrKeyNotSupported if invoked on a KVStore instance before v2.
	GetExecutionComponentVersion() (MagnitudeVersion, error)
	// GetExecutionComponentVersionUpgrade returns the most recent upgrade for the Execution Component Version,
	// if one exists (otherwise returns nil). The upgrade will be returned even if it has already been applied.
	// Returns nil if invoked on a KVStore instance before v2.
	GetExecutionComponentVersionUpgrade() *ViewBasedActivator[MagnitudeVersion]

	// GetExecutionMeteringParameters returns the Execution metering parameters.
	// Error Returns:
	//   - kvstore.ErrKeyNotSupported if invoked on a KVStore instance before v2.
	GetExecutionMeteringParameters() (ExecutionMeteringParameters, error)
	// GetExecutionMeteringParametersUpgrade returns the most recent upgrade for the Execution Metering Parameters,
	// if one exists (otherwise returns nil). The upgrade will be returned even if it has already been applied.
	// Returns nil if invoked on a KVStore instance before v2.
	GetExecutionMeteringParametersUpgrade() *ViewBasedActivator[ExecutionMeteringParameters]
}

// ExecutionMeteringParameters are used to measure resource usage of transactions,
// which affects fee calculations and transaction/script stopping conditions.
type ExecutionMeteringParameters struct {
	// ExecutionEffortWeights ...
	ExecutionEffortWeights map[uint64]uint64
	// ExecutionMemoryWeights ...
	ExecutionMemoryWeights map[uint64]uint64
	// ExecutionMemoryLimit ...
	ExecutionMemoryLimit uint64
}

func DefaultExecutionMeteringParameters() ExecutionMeteringParameters {
	return ExecutionMeteringParameters{
		ExecutionEffortWeights: make(map[uint64]uint64),
		ExecutionMemoryWeights: make(map[uint64]uint64),
		ExecutionMemoryLimit:   math.MaxUint64,
	}
}

// EncodeRLP defines RLP encoding behaviour for ExecutionMeteringParameters, overriding the default behaviour.
// We convert maps to ordered slices of key-pairs before encoding, because RLP does not directly support maps.
// We require this KVStore field type to be RLP-encodable so we can compute the hash/ID of a kvstore model instance.
func (params *ExecutionMeteringParameters) EncodeRLP(w io.Writer) error {
	type pair struct {
		Key   uint64
		Value uint64
	}
	pairOrdering := func(a, b pair) int {
		if a.Key < b.Key {
			return -1
		}
		if a.Key > b.Key {
			return 1
		}
		// Since we are ordering by key taken directly from a single Go map type, it is not possible to
		// observe two identical keys while ordering. If we do, some invariant has been violated.
		// Also, since the sort used is non-stable, this could result in non-deterministic hashes.
		panic("critical invariant violated: map with duplicate keys")
	}

	orderedEffortParams := make([]pair, 0, len(params.ExecutionEffortWeights))
	for k, v := range params.ExecutionEffortWeights {
		orderedEffortParams = append(orderedEffortParams, pair{k, v})
	}
	slices.SortFunc(orderedEffortParams, pairOrdering)

	orderedMemoryParams := make([]pair, 0, len(params.ExecutionMemoryWeights))
	for k, v := range params.ExecutionMemoryWeights {
		orderedMemoryParams = append(orderedMemoryParams, pair{k, v})
	}
	slices.SortFunc(orderedMemoryParams, pairOrdering)

	return rlp.Encode(w, struct {
		ExecutionEffortWeights []pair
		ExecutionMemoryWeights []pair
		ExecutionMemoryLimit   uint64
	}{
		ExecutionEffortWeights: orderedEffortParams,
		ExecutionMemoryWeights: orderedMemoryParams,
		ExecutionMemoryLimit:   params.ExecutionMemoryLimit,
	})
}

// IsUndefined returns true if params is semantically undefined.
// TODO remove
func (params *ExecutionMeteringParameters) IsUndefined() bool {
	return params.ExecutionEffortWeights == nil && params.ExecutionMemoryWeights == nil && params.ExecutionMemoryLimit == 0
}

// UndefinedExecutionMeteringParameters represents the zero or unset value for ExecutionMeteringParameters.
// TODO remove
var UndefinedExecutionMeteringParameters = ExecutionMeteringParameters{}

// VersionedEncodable defines the interface for a versioned key-value store independent
// of the set of keys which are supported. This allows the storage layer to support
// storing different key-value model versions within the same software version.
type VersionedEncodable interface {
	// VersionedEncode encodes the key-value store, returning the version separately
	// from the encoded bytes.
	// No errors are expected during normal operation.
	VersionedEncode() (uint64, []byte, error)
}

// ViewBasedActivator represents a scheduled update to some protocol parameter P.
// (The relationship between a ViewBasedActivator and P is managed outside this model.)
// Once the ViewBasedActivator A is persisted to the protocol state, P is updated to value
// A.Data in the first block with view ≥ A.ActivationView (in each fork independently).
type ViewBasedActivator[T any] struct {
	// Data is the pending new value, to be applied when reaching or exceeding ActivationView.
	Data T
	// ActivationView is the view at which the new value should be applied.
	ActivationView uint64
}

// UpdatableField represents a protocol parameter which can be updated using a ViewBasedActivator.
type UpdatableField[T any] struct {
	// CurrentValue is the value that is active after constructing the block
	// that this Protocol State pertains to.
	CurrentValue T

	// Update is optional and is nil if no value update has been scheduled yet.
	// This field will hold the last scheduled update until a newer update
	// directive is received, even if the value update has already happened.
	// The update should be applied when reaching or exceeding the ActivationView.
	Update *ViewBasedActivator[T]
}

// MagnitudeVersion is intended as an intuitive representation of the “magnitude of change”.
//
// # CAUTION: Don't confuse this with semver!
//
// This versioning representation DEVIATES from established Semantic Versioning.
// Any two different versions of the Execution Stack are considered incompatible.
// In particular, two versions only differing in their minor, might be entirely downwards-INCOMPATIBLE.
//
// We generally recommend to use Integer Versioning for components. The MagnitudeVersion scheme should
// be only used when there is a clear advantage over Integer Versioning, which outweighs the risk of falsely
// making compatibility assumptions by confusing this scheme with Semantic Versioning!
//
// MagnitudeVersion helps with an intuitive representation of the “magnitude of change”.
// For example, for the execution stack, bug fixes closing unexploited edge-cases will be a relatively
// frequent cause of upgrades. Those bug fixes could be reflected by minor version bumps, whose
// imperfect downwards compatibility might frequently suffice to warrant Access Nodes using the same
// version (higher minor) across version boundaries. In comparison, major version change would generally
// indicate broader non-compatibility (or larger feature additions) where it is very unlikely that the Access
// Node can use one implementation for versions with different major.
//
// We emphasize again that this differentiation of “imperfect but good-enough downwards compatibility”
// is in no way reflected by the versioning scheme. Any automated decisions regarding compatibility of
// different versions are to be avoided (including versions where only the minor is different).
//
// Engineering teams using this scheme must be aware that the MagnitudeVersion is easily
// misleading wrt to incorrect assumptions about downwards compatibility. Avoiding problems (up to and
// including the possibility of mainnet outages) requires continued awareness of all engineers in the
// teams working with this version. The engineers in those teams must commit to diligently documenting
// all relevant changes, details regarding magnitude of changes and if applicable “imperfect but
// good-enough downwards compatibility”.
type MagnitudeVersion struct {
	Major uint
	Minor uint
}

// UndefinedMagnitudeOfChangeVersion represents the zero or unset value for a MagnitudeVersion.
var UndefinedMagnitudeOfChangeVersion = MagnitudeVersion{}
