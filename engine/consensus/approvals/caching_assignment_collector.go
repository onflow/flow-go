package approvals

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/consensus"
	"github.com/onflow/flow-go/model/flow"
)

// CachingAssignmentCollector is an AssignmentCollectorState with the fixed `ProcessingStatus` of `CachingApprovals`.
type CachingAssignmentCollector struct {
	AssignmentCollectorBase

	log            zerolog.Logger
	approvalsCache *ApprovalsCache           // in-memory cache of approvals (not-verified)
	incResCache    *IncorporatedResultsCache // in-memory cache for incorporated results that were processed
}

func NewCachingAssignmentCollector(collectorBase AssignmentCollectorBase) *CachingAssignmentCollector {
	return &CachingAssignmentCollector{
		AssignmentCollectorBase: collectorBase,
		log:                     collectorBase.log.With().Str("component", "caching_assignment_collector").Logger(),
		approvalsCache:          NewApprovalsCache(0),
		incResCache:             NewIncorporatedResultsCache(0),
	}
}

func (ac *CachingAssignmentCollector) ProcessingStatus() ProcessingStatus { return CachingApprovals }
func (ac *CachingAssignmentCollector) CheckEmergencySealing(consensus.SealingObservation, uint64) error {
	return nil
}
func (ac *CachingAssignmentCollector) RequestMissingApprovals(consensus.SealingObservation, uint64) (uint, error) {
	return 0, nil
}

// ProcessIncorporatedResult starts tracking the approval for IncorporatedResult.
// Method is idempotent.
// Error Returns:
//  * no errors expected during normal operation;
//    errors might be symptoms of bugs or internal state corruption (fatal)
func (ac *CachingAssignmentCollector) ProcessIncorporatedResult(incorporatedResult *flow.IncorporatedResult) error {
	// check that result is the one that this VerifyingAssignmentCollector manages
	if resID := incorporatedResult.Result.ID(); resID != ac.ResultID() {
		return fmt.Errorf("this VerifyingAssignmentCollector manages result %x but got %x", ac.ResultID(), resID)
	}

	// In case the result is already cached, we first read the cache.
	// This is much cheaper than attempting to write right away.
	irID := incorporatedResult.ID()
	if cached := ac.incResCache.Get(irID); cached != nil {
		return nil
	}
	ac.incResCache.Put(irID, incorporatedResult)
	return nil
}

// ProcessApproval ingests Result Approvals and triggers sealing of execution result
// when sufficient approvals have arrived.
// Error Returns:
//  * nil in case of success (outdated approvals might be silently discarded)
//  * engine.InvalidInputError if the result approval is invalid
//  * any other errors might be symptoms of bugs or internal state corruption (fatal)
func (ac *CachingAssignmentCollector) ProcessApproval(approval *flow.ResultApproval) error {
	ac.log.Debug().
		Str("result_id", approval.Body.ExecutionResultID.String()).
		Str("verifier_id", approval.Body.ApproverID.String()).
		Msg("processing result approval")

	// check that approval is for the expected result to reject incompatible inputs
	if approval.Body.ExecutionResultID != ac.ResultID() {
		return fmt.Errorf("this CachingAssignmentCollector processes only approvals for result (%x) but got an approval for (%x)", ac.resultID, approval.Body.ExecutionResultID)
	}
	// approval has to refer same block as execution result
	if approval.Body.BlockID != ac.BlockID() {
		return engine.NewInvalidInputErrorf("result approval for invalid block, expected (%x) vs (%x)",
			ac.BlockID(), approval.Body.BlockID)
	}

	// if we have this approval cached already, no need to process it again
	approvalCacheID := approval.Body.PartialID()
	if cached := ac.approvalsCache.Get(approvalCacheID); cached != nil {
		return nil
	}
	ac.approvalsCache.Put(approvalCacheID, approval)
	return nil
}

func (ac *CachingAssignmentCollector) GetIncorporatedResults() []*flow.IncorporatedResult {
	return ac.incResCache.All()
}

func (ac *CachingAssignmentCollector) GetApprovals() []*flow.ResultApproval {
	return ac.approvalsCache.All()
}
