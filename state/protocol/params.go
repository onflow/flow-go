package protocol

import (
	"github.com/onflow/flow-go/model/flow"
)

// Params are parameters of the protocol state, divided into parameters of
// this specific instance of the state (varies from node to node) and global
// parameters of the state.
type Params interface {
	InstanceParams
	GlobalParams
}

// InstanceParams represents protocol state parameters that vary between instances.
// For example, two nodes both running in the same spork on Flow Mainnet may have
// different instance params.
type InstanceParams interface {

	// FinalizedRoot returns the finalized root header of the current protocol state. This will be
	// the head of the protocol state snapshot used to bootstrap this state and
	// may differ from node to node for the same protocol state.
	// No errors are expected during normal operation.
	FinalizedRoot() *flow.Header

	// SealedRoot returns the sealed root block. If it's different from FinalizedRoot() block,
	// it means the node is bootstrapped from mid-spork.
	// No errors are expected during normal operation.
	SealedRoot() *flow.Header

	// Seal returns the root block seal of the current protocol state. This will be
	// the seal for the root block used to bootstrap this state and may differ from
	// node to node for the same protocol state.
	// No errors are expected during normal operation.
	Seal() *flow.Seal

	// EpochFallbackTriggered returns whether epoch fallback mode (EECC) has been triggered.
	// EECC is a permanent, spork-scoped state which is triggered when the next
	// epoch fails to be committed in the allocated time. Once EECC is triggered,
	// it will remain in effect until the next spork.
	// No errors are expected during normal operation.
	EpochFallbackTriggered() (bool, error)
}

// GlobalParams represents protocol state parameters that do not vary between instances.
// Any nodes running in the same spork, on the same network (same chain ID) must
// have the same global params.
type GlobalParams interface {

	// ChainID returns the chain ID for the current Flow network. The chain ID
	// uniquely identifies a Flow network in perpetuity across epochs and sporks.
	ChainID() flow.ChainID

	// SporkID returns the unique identifier for this network within the current spork.
	// This ID is determined at the beginning of a spork during bootstrapping and is
	// part of the root protocol state snapshot.
	SporkID() flow.Identifier

	// SporkRootBlockHeight returns the height of the spork's root block.
	// This value is determined at the beginning of a spork during bootstrapping.
	// If node uses a sealing segment for bootstrapping then this value will be carried over
	// as part of snapshot.
	SporkRootBlockHeight() uint64

	// ProtocolVersion returns the protocol version, the major software version
	// of the protocol software.
	ProtocolVersion() uint

	// EpochCommitSafetyThreshold defines a deadline for sealing the EpochCommit
	// service event near the end of each epoch - the "epoch commitment deadline".
	// Given a safety threshold t, the deadline for an epoch with final view f is:
	//   Epoch Commitment Deadline: d=f-t
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
	// HAPPY PATH: If an EpochCommit service event has been sealed w.r.t. B, no action is taken.
	// FALLBACK PATH: If no EpochCommit service event has been sealed w.r.t. B, epoch fallback mode (EECC) is triggered.
	//
	// CONTEXT:
	// The epoch commitment deadline exists to ensure that all nodes agree on
	// whether epoch fallback mode is triggered for a particular epoch, before
	// the epoch actually ends. Although the use of this deadline DOES NOT
	// guarantee these properties, it is a simpler way to assure them with high
	// likelihood, given reasonable configuration.
	// In particular, all nodes will agree about EECC being triggered (or not)
	// if at least one block with view in [d, f] is finalized - in other words
	// at least one block is finalized after the epoch commitment deadline, and
	// before the next epoch begins.
	//
	// When selecting a threshold value, ensure:
	// * The deadline is after the end of the DKG, with enough buffer between
	//   the two that the EpochCommit event is overwhelmingly likely to be emitted
	//   before the deadline, if it is emitted at all.
	// * The buffer between the deadline and the final view of the epoch is large
	//   enough that the network is overwhelming likely to finalize at least one
	//   block with a view in this range
	//
	//                /- Epoch Commitment Deadline
	// EPOCH N        v        EPOCH N+1
	// ...------------|------||-----...
	//
	EpochCommitSafetyThreshold() uint64
}
