package tracker

import (
	"github.com/onflow/flow-go/engine/consensus"
	"github.com/onflow/flow-go/model/flow"
)

// NoopSealingTracker implements the sealing.SealingTracker and sealing.SealingObservation interfaces.
// By using the same instance, we avoid GC overhead. All methods are essentially NoOps.
type NoopSealingTracker struct{}

func (t *NoopSealingTracker) NewSealingObservation(*flow.Header, *flow.Seal, *flow.Header) consensus.SealingObservation {
	return t
}

func (t *NoopSealingTracker) QualifiesForEmergencySealing(*flow.IncorporatedResult, bool) {}
func (t *NoopSealingTracker) ApprovalsRequested(*flow.IncorporatedResult, uint)           {}
func (t *NoopSealingTracker) Complete()                                                   {}
func (t *NoopSealingTracker) ApprovalsMissing(*flow.IncorporatedResult, map[uint64]flow.IdentifierList) {
}
