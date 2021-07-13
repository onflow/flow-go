package consensus

import "github.com/onflow/flow-go/model/flow"

// SealingTracker is an auxiliary component for tracking progress of the sealing
// logic (specifically sealing.Core). It has access to the storage, to collect data
// that is not be available directly from sealing.Core. The SealingTracker is immutable
// and therefore intrinsically thread safe.
//
// The SealingTracker essentially acts as a factory for individual SealingObservations,
// which capture information about the progress of a _single_ go routine. Consequently,
// SealingObservations don't need to be concurrency safe, as they are supposed to
// be thread-local structure.
type SealingTracker interface {

	// NewSealingObservation constructs a SealingObservation, which capture information
	// about the progress of a _single_ go routine. Consequently, SealingObservations
	// don't need to be concurrency safe, as they are supposed to be thread-local structure.
	NewSealingObservation(finalizedBlock *flow.Header, seal *flow.Seal, sealedBlock *flow.Header) SealingObservation
}

// SealingObservation captures information about the progress of a _single_ go routine.
// Consequently, it is _not concurrency safe_, as SealingObservation is intended to be
// a thread-local structure.
// SealingObservation is supposed to track the status of various (unsealed) incorporated
// results, which sealing.Core processes (driven by that single goroutine).
type SealingObservation interface {

	// QualifiesForEmergencySealing captures whether sealing.Core has
	// determined that the incorporated result qualifies for emergency sealing.
	QualifiesForEmergencySealing(ir *flow.IncorporatedResult, emergencySealable bool)

	// ApprovalsMissing captures whether sealing.Core has determined that
	// some approvals are still missing for the incorporated result. Calling this
	// method with empty `chunksWithMissingApprovals` indicates that all chunks
	// have sufficient approvals.
	ApprovalsMissing(ir *flow.IncorporatedResult, chunksWithMissingApprovals map[uint64]flow.IdentifierList)

	// ApprovalsRequested captures the number of approvals that the business
	// logic has re-requested for the incorporated result.
	ApprovalsRequested(ir *flow.IncorporatedResult, requestCount uint)

	// Complete is supposed to be called when a single execution of the sealing logic
	// has been completed. It compiles the information about the incorporated results.
	Complete()
}
