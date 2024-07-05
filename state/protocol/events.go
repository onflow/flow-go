package protocol

import (
	"github.com/onflow/flow-go/model/flow"
)

const (
	EpochExtendedEvtType              = "EpochExtended"
	EpochCommittedPhaseStartedEvtType = "EpochCommittedPhaseStarted"
)

// EpochCommittedPhaseStartedData EpochCommittedPhaseStarted event data.
type EpochCommittedPhaseStartedData struct {
	// EpochCounter is the current epoch counter.
	CurrentEpochCounter uint64
	// First header of the committed epoch.
	First *flow.Header
}

// EpochExtendedData EpochExtended event data.
type EpochExtendedData struct {
	// EpochCounter is the current epoch counter at the block when EFM was triggered.
	EpochCounter uint64
	// Header is the block when EFM was triggered
	Header *flow.Header
	// Extension the epoch extension.
	Extension *flow.EpochExtension
}

// Consumer defines a set of events that occur within the protocol state, that
// can be propagated to other components via an implementation of this interface.
// Collectively, these are referred to as "Protocol Events".
//
// Protocol events are delivered immediately after the database transaction
// committing the corresponding state change completes successfully.
// This means that events are delivered exactly once, while the system is running.
// Events may not be delivered during crashes and restarts, but any missed events
// are guaranteed to be reflected in the Protocol State upon restarting.
// Components consuming protocol events which cannot tolerate missed events
// must implement initialization logic which accounts for any missed events.
//
// EXAMPLE:
// Suppose block A is finalized at height 100. If the BlockFinalized(A) event is
// dropped due to a crash, then when the node restarts, the latest finalized block
// in the Protocol State is guaranteed to be A.
//
// CAUTION: Protocol event subscriber callbacks are invoked synchronously in the
// critical path of protocol state mutations. Most subscribers should immediately
// spawn a goroutine to handle the notification to avoid blocking protocol state
// progression, especially for frequent protocol events (eg. BlockFinalized).
//
// NOTE: the epoch-related callbacks are only called once the fork containing
// the relevant event has been finalized.
type Consumer interface {
	// BlockFinalized is called when a block is finalized.
	// Formally, this callback is informationally idempotent. I.e. the consumer
	// of this callback must handle repeated calls for the same block.
	BlockFinalized(block *flow.Header)

	// BlockProcessable is called when a correct block is encountered that is
	// ready to be processed (i.e. it is connected to the finalized chain and
	// its source of randomness is available).
	// BlockProcessable provides the block and a certifying QC. BlockProcessable is never emitted
	// for the root block, as the root block is always processable.
	// Formally, this callback is informationally idempotent. I.e. the consumer
	// of this callback must handle repeated calls for the same block.
	BlockProcessable(block *flow.Header, certifyingQC *flow.QuorumCertificate)

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
	// Referencing the diagram below, the event is emitted when block b is finalized.
	// The block parameter is the first block of the epoch setup phase (block b).
	//
	// |<-- Epoch N ------------------------------------------------->|
	// |<-- StakingPhase -->|<-- SetupPhase -->|<-- CommittedPhase -->|
	//                    ^--- block A - this block's execution result contains an EpochSetup event
	//                      ^--- block b - contains seal for block A, first block of Setup phase
	//                         ^--- block c - finalizes block b, triggers EpochSetupPhaseStarted event
	//
	//	- data the EpochExtendedData.
	// NOTE: Only called once the phase transition has been finalized.
	EpochSetupPhaseStarted(currentEpochCounter uint64, first *flow.Header)

	// EpochCommittedPhaseStarted is called when we begin the epoch committed phase
	// for the current epoch. This is equivalent to the end of the epoch setup
	// phase for the current epoch.
	//
	// Referencing the diagram below, the event is emitted when block e is finalized.
	// The block parameter is the first block of the epoch committed phase (block e).
	//
	// |<-- Epoch N ------------------------------------------------->|
	// |<-- StakingPhase -->|<-- SetupPhase -->|<-- CommittedPhase -->|
	//                                       ^--- block D - this block's execution result contains an EpochCommit event
	//                                         ^--- block e - contains seal for block D, first block of Committed phase
	//                                            ^--- block f - finalizes block e, triggers EpochCommittedPhaseStarted event
	//
	// NOTE: Only called once the phase transition has been finalized.
	EpochCommittedPhaseStarted(data *EpochCommittedPhaseStartedData)

	// EpochFallbackModeTriggered is called when Epoch Fallback Mode [EFM] is triggered.
	// EFM is triggered when an invalid or unexpected epoch-related service event is observed,
	// or an expected service event is not observed before the epoch commitment deadline.
	// After EFM is triggered, we drop any potentially pending but uncommitted future epoch setup.
	// When an EpochRecover event is observed, regular epoch transitions begin again.
	// Usually, this means we remain in the current epoch until EFM is exited.
	// If EFM was triggered within the EpochCommitted phase, then we complete the transition
	// to the next, already-committed epoch, then remain in that epoch until EFM is exited.
	// Consumers can get context for handling events from:
	//   - epochCounter is the current epoch counter at the block when EFM was triggered
	//   - header is the block when EFM was triggered
	//
	// NOTE: This notification is emitted when the block triggering EFM is finalized.
	EpochFallbackModeTriggered(epochCounter uint64, header *flow.Header)

	// EpochFallbackModeExited is called when epoch fallback mode [EFM] is exited.
	// EFM is exited when an EpochRecover service event is processed, which defines
	// a final view for the current epoch and fully specifies the subsequent epoch.
	// Consumers can get context for handling events from:
	//   - epochCounter is the current epoch counter at the block when EFM was triggered
	//   - header is the block when EFM was triggered
	//
	// NOTE: Only called once the block incorporating the EpochRecover is finalized.
	EpochFallbackModeExited(epochCounter uint64, header *flow.Header)

	// EpochExtended is called when a flow.EpochExtension is added to the current epoch
	// Consumers can get context for handling events from:
	//   - data the EpochExtendedData.
	//
	// NOTE: This notification is emitted when the block triggering the EFM extension is finalized.
	EpochExtended(data *EpochExtendedData)
}
