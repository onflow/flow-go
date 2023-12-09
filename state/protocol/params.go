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
	FinalizedRoot() *flow.Header

	// SealedRoot returns the sealed root block. If it's different from FinalizedRoot() block,
	// it means the node is bootstrapped from mid-spork.
	SealedRoot() *flow.Header

	// Seal returns the root block seal of the current protocol state. This will be
	// the seal for the root block used to bootstrap this state and may differ from
	// node to node for the same protocol state.
	Seal() *flow.Seal

	// EpochFallbackTriggered returns whether Epoch Fallback Mode [EFM] has been triggered.
	// EFM is a permanent, spork-scoped state which is triggered when the next
	// epoch fails to be committed in the allocated time. Once EFM is triggered,
	// it will remain in effect until the next spork.
	// TODO for 'leaving Epoch Fallback via special service event'
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

	// EpochCommitSafetyThreshold [t] defines a deadline for sealing the EpochCommit
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
	EpochCommitSafetyThreshold() uint64
}
