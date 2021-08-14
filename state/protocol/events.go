package protocol

import (
	"github.com/onflow/flow-go/model/flow"
)

// Consumer defines a set of events that occur within the protocol state, that
// can be propagated to other components via an implementation of this interface.
// Consumer implementations must be non-blocking.
//
// NOTE: the epoch-related callbacks are only called once the fork containing
// the relevant event has been finalized.
type Consumer interface {

	// BlockFinalized is called when a block is finalized.
	// Formally, this callback is informationally idempotent. I.e. the consumer
	// of this callback must handle repeated calls for the same block.
	BlockFinalized(block *flow.Header)

	// BlockProcessable is called when a correct block is encountered
	// that is ready to be processed (i.e. it is connected to the finalized
	// chain and its source of randomness is available).
	// Formally, this callback is informationally idempotent. I.e. the consumer
	// of this callback must handle repeated calls for the same block.
	BlockProcessable(block *flow.Header)

	// EpochTransition is called when we transition to a new epoch. This is
	// equivalent to the beginning of the new epoch's staking phase and the end
	// of the previous epoch's epoch committed phase.
	//
	// The block parameter is the first block of the new epoch.
	//
	// NOTE: Only called once the transition has been finalized.
	EpochTransition(newEpochCounter uint64, first *flow.Header)

	// EpochSetupPhaseStarted is called when we begin the epoch setup phase for
	// the current epoch. This is equivalent to the end of the epoch staking
	// phase for the current epoch.
	//
	// Referencing the diagram below, the event is emitted when block c is incorporated.
	// The block parameter is the first block of the epoch setup phase (block c).
	//
	// |<-- Epoch N ------------------------------------------------->|
	// |<-- StakingPhase -->|<-- SetupPhase -->|<-- CommittedPhase -->|
	//                    ^--- block A - this block's execution result contains an EpochSetup event
	//                      ^--- block b - contains seal for block A
	//                         ^--- block c - contains qc for block b, first block of Setup phase
	//                           ^--- block d - finalizes block c, triggers EpochSetupPhaseStarted event
	//
	// NOTE: Only called once the phase transition has been finalized.
	EpochSetupPhaseStarted(currentEpochCounter uint64, first *flow.Header)

	// EpochCommittedPhaseStarted is called when we begin the epoch committed phase
	// for the current epoch. This is equivalent to the end of the epoch setup
	// phase for the current epoch.
	//
	// Referencing the diagram below, the event is emitted when block f is received.
	// The block parameter is the first block of the epoch committed phase (block f).
	//
	// |<-- Epoch N ------------------------------------------------->|
	// |<-- StakingPhase -->|<-- SetupPhase -->|<-- CommittedPhase -->|
	//                                       ^--- block D - this block's execution result contains an EpochCommit event
	//                                         ^--- block e - contains seal for block D
	//                                            ^--- block f - contains qc for block e, first block of Committed phase
	//                                              ^--- block g - finalizes block f, triggers EpochCommittedPhaseStarted event
	///
	//
	// NOTE: Only called once the phase transition has been finalized.
	EpochCommittedPhaseStarted(currentEpochCounter uint64, first *flow.Header)
}
