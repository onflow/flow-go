package protocol

import (
	"github.com/onflow/flow-go/model/flow"
)

// Consumer defines a set of events that occur within the protocol state, that
// can be propagated to other components via an implementation of this interface.
// Consumer implementations must be non-blocking.
//
// NOTE: These methods are only called once the fork containing the relevant
// event has been finalized.
type Consumer interface {

	// BlockFinalized is called when a block is finalized.
	BlockFinalized(block *flow.Header)

	// EpochTransition is called when we transition to a new epoch. This is
	// equivalent to the beginning of the new epoch's staking phase and the end
	// of the previous epoch's epoch committed phase.
	//
	// The block parameter is the first block of the new epoch.
	//
	// NOTE: Only called once the transition has been finalized.
	EpochTransition(newEpoch uint64, first *flow.Header)

	// EpochSetupPhaseStarted is called when we begin the epoch setup phase for
	// the current epoch. This is equivalent to the end of the epoch staking
	// phase for the current epoch.
	//
	// Referencing the diagram below, the event is emitted when block c is incorporated.
	// The block parameter is the first block of the epoch setup phase (block b).
	//
	// |<-- Epoch N ------------------------------------------------->|
	// |<-- StakingPhase -->|<-- SetupPhase -->|<-- CommittedPhase -->|
	//                    ^--- block A - this block's execution result contains an EpochSetup event
	//                      ^--- block b - contains seal for block A, first block of Setup phase
	//                         ^--- block c - finalizes block b and triggers `EpochSetupPhaseStarted`
	//
	// NOTE: Only called once the phase transition has been finalized.
	EpochSetupPhaseStarted(epoch uint64, first *flow.Header)

	// EpochCommittedPhase is called when we begin the epoch committed phase
	// for the current epoch. This is equivalent to the end of the epoch setup
	// phase for the current epoch.
	//
	// Referencing the diagram below, the event is emitted when block f is received.
	// The block parameter is the first block of the epoch committed phase (block e).
	//
	// |<-- Epoch N ------------------------------------------------->|
	// |<-- StakingPhase -->|<-- SetupPhase -->|<-- CommittedPhase -->|
	//                                       ^--- block D - this block's execution result contains an EpochCommit event
	//                                         ^--- block e - contains seal for block D, first block of Committed phase
	//                                            ^--- block f - this block finalizes block e, and triggers `EpochCommittedPhaseStarted`
	///
	//
	// NOTE: Only called once the phase transition has been finalized.
	EpochCommittedPhaseStarted(epoch uint64, first *flow.Header)
}
