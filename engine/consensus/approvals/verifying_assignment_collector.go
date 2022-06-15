package approvals

import (
	"fmt"
	"math/rand"
	"sync"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/consensus"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/state/protocol"
)

// DefaultEmergencySealingThreshold is the default number of blocks which indicates that ER should be sealed using emergency
// sealing.
const DefaultEmergencySealingThreshold = 100

// VerifyingAssignmentCollector
// Context:
//  * When the same result is incorporated in multiple different forks,
//    unique verifier assignment is determined for each fork.
//  * The assignment collector is intended to encapsulate the known
//    assignments for a particular execution result.
// VerifyingAssignmentCollector has a strict ordering of processing, before processing
// approvals at least one incorporated result has to be processed.
// VerifyingAssignmentCollector takes advantage of internal caching to speed up processing approvals for different assignments
// VerifyingAssignmentCollector is responsible for validating approvals on result-level (checking signature, identity).
type VerifyingAssignmentCollector struct {
	AssignmentCollectorBase

	log                    zerolog.Logger
	lock                   sync.RWMutex
	collectors             map[flow.Identifier]*ApprovalCollector // collectors is a mapping IncorporatedBlockID -> ApprovalCollector
	authorizedApprovers    map[flow.Identifier]*flow.Identity     // map of approvers pre-selected at block that is being sealed
	verifiedApprovalsCache *ApprovalsCache                        // in-memory cache of approvals (already verified)
}

// NewVerifyingAssignmentCollector instantiates a new VerifyingAssignmentCollector.
// All errors are unexpected and potential symptoms of internal bugs or state corruption (fatal).
func NewVerifyingAssignmentCollector(collectorBase AssignmentCollectorBase) (*VerifyingAssignmentCollector, error) {
	// pre-select all authorized verifiers at the block that is being sealed
	authorizedApprovers, err := authorizedVerifiersAtBlock(collectorBase.state, collectorBase.BlockID())
	if err != nil {
		return nil, fmt.Errorf("could not determine authorized verifiers for sealing candidate: %w", err)
	}
	numberChunks := collectorBase.result.Chunks.Len()

	return &VerifyingAssignmentCollector{
		AssignmentCollectorBase: collectorBase,
		log:                     collectorBase.log.With().Str("component", "verifying_assignment_collector").Logger(),
		lock:                    sync.RWMutex{},
		collectors:              make(map[flow.Identifier]*ApprovalCollector),
		authorizedApprovers:     authorizedApprovers,
		verifiedApprovalsCache:  NewApprovalsCache(uint(numberChunks * len(authorizedApprovers))),
	}, nil
}

func (ac *VerifyingAssignmentCollector) collectorByBlockID(incorporatedBlockID flow.Identifier) *ApprovalCollector {
	ac.lock.RLock()
	defer ac.lock.RUnlock()
	return ac.collectors[incorporatedBlockID]
}

// emergencySealable determines whether an incorporated Result qualifies for "emergency sealing".
// ATTENTION: this is a temporary solution, which is NOT BFT compatible. When the approval process
// hangs far enough behind finalization (measured in finalized but unsealed blocks), emergency
// sealing kicks in. This will be removed when implementation of Sealing & Verification is finished.
func (ac *VerifyingAssignmentCollector) emergencySealable(collector *ApprovalCollector, finalizedBlockHeight uint64) bool {
	// Criterion for emergency sealing:
	// there must be at least DefaultEmergencySealingThreshold number of blocks between
	// the block that _incorporates_ result and the latest finalized block
	return collector.IncorporatedBlock().Height+DefaultEmergencySealingThreshold <= finalizedBlockHeight
}

// CheckEmergencySealing checks the managed assignments whether their result can be emergency
// sealed. Seals the results where possible.
func (ac *VerifyingAssignmentCollector) CheckEmergencySealing(observer consensus.SealingObservation, finalizedBlockHeight uint64) error {
	for _, collector := range ac.allCollectors() {
		sealable := ac.emergencySealable(collector, finalizedBlockHeight)
		observer.QualifiesForEmergencySealing(collector.IncorporatedResult(), sealable)
		if sealable {
			err := collector.SealResult()
			if err != nil {
				return fmt.Errorf("could not create emergency seal for result %x incorporated at %x: %w",
					ac.ResultID(), collector.IncorporatedBlockID(), err)
			}
		}
	}

	return nil
}

func (ac *VerifyingAssignmentCollector) ProcessingStatus() ProcessingStatus {
	return VerifyingApprovals
}

// ProcessIncorporatedResult starts tracking the approval for IncorporatedResult.
// Method is idempotent.
// Error Returns:
//  * no errors expected during normal operation;
//    errors might be symptoms of bugs or internal state corruption (fatal)
func (ac *VerifyingAssignmentCollector) ProcessIncorporatedResult(incorporatedResult *flow.IncorporatedResult) error {
	ac.log.Debug().
		Str("result_id", incorporatedResult.Result.ID().String()).
		Str("incorporated_block_id", incorporatedResult.IncorporatedBlockID.String()).
		Str("block_id", incorporatedResult.Result.BlockID.String()).
		Msg("processing incorporated result")

	// check that result is the one that this VerifyingAssignmentCollector manages
	if irID := incorporatedResult.Result.ID(); irID != ac.ResultID() {
		return fmt.Errorf("this VerifyingAssignmentCollector manages result %x but got %x", ac.ResultID(), irID)
	}

	// NoOp, if we already have a collector for this incorporatedResult
	incorporatedBlockID := incorporatedResult.IncorporatedBlockID
	if collector := ac.collectorByBlockID(incorporatedBlockID); collector != nil {
		return nil
	}

	// Constructing ApprovalCollector for IncorporatedResult
	// The VerifyingAssignmentCollector is not locked while instantiating the ApprovalCollector. Hence, it is possible that
	// multiple threads simultaneously compute the verifier assignment. Nevertheless, the implementation is safe in
	// that only one of the instantiated ApprovalCollectors will be stored in the cache. In terms of locking duration,
	// it's better to perform extra computation in edge cases than lock this logic with a mutex,
	// since it's quite unlikely that same incorporated result will be processed by multiple goroutines simultaneously.
	assignment, err := ac.assigner.Assign(incorporatedResult.Result, incorporatedBlockID)
	if err != nil {
		return fmt.Errorf("could not determine chunk assignment: %w", err)
	}
	incorporatedBlock, err := ac.headers.ByBlockID(incorporatedBlockID)
	if err != nil {
		return fmt.Errorf("failed to retrieve header of incorporated block %s: %w",
			incorporatedBlockID, err)
	}
	executedBlock, err := ac.headers.ByBlockID(incorporatedResult.Result.BlockID)
	if err != nil {
		return fmt.Errorf("failed to retrieve header of incorporatedResult %s: %w",
			incorporatedResult.Result.BlockID, err)
	}
	collector, err := NewApprovalCollector(ac.log, incorporatedResult, incorporatedBlock, executedBlock, assignment, ac.seals, ac.requiredApprovalsForSealConstruction)
	if err != nil {
		return fmt.Errorf("instantiation of ApprovalCollector failed: %w", err)
	}

	// Now, we add the ApprovalCollector to the VerifyingAssignmentCollector:
	// no-op if an ApprovalCollector has already been added by a different routine
	isDuplicate := ac.putCollector(incorporatedBlockID, collector)
	if isDuplicate {
		return nil
	}

	// process approvals that have passed needed checks and are ready to be processed
	for _, approval := range ac.verifiedApprovalsCache.All() {
		// those approvals are verified already and shouldn't yield any errors
		err = collector.ProcessApproval(approval)
		if err != nil {
			return fmt.Errorf("processing already validated approval %x failed: %w", approval.ID(), err)
		}
	}

	return nil
}

// putCollector stores the collector if it is not already present in the collectors map
// and returns false (no duplicate). NoOp if a collector for the incorporatedBlockID is
// already stored, in which case true is returned (indicating a duplicate).
func (ac *VerifyingAssignmentCollector) putCollector(incorporatedBlockID flow.Identifier, collector *ApprovalCollector) bool {
	ac.lock.Lock()
	defer ac.lock.Unlock()
	if _, ok := ac.collectors[incorporatedBlockID]; ok {
		return true
	}
	ac.collectors[incorporatedBlockID] = collector
	return false
}

func (ac *VerifyingAssignmentCollector) allCollectors() []*ApprovalCollector {
	ac.lock.RLock()
	defer ac.lock.RUnlock()
	collectors := make([]*ApprovalCollector, 0, len(ac.collectors))
	for _, collector := range ac.collectors {
		collectors = append(collectors, collector)
	}
	return collectors
}

func (ac *VerifyingAssignmentCollector) verifyAttestationSignature(approval *flow.ResultApprovalBody, nodeIdentity *flow.Identity) error {
	id := approval.Attestation.ID()
	valid, err := nodeIdentity.StakingPubKey.Verify(approval.AttestationSignature, id[:], ac.sigHasher)
	if err != nil {
		return fmt.Errorf("failed to verify attestation signature: %w", err)
	}

	if !valid {
		return engine.NewInvalidInputErrorf("invalid attestation signature for (%x)", nodeIdentity.NodeID)
	}

	return nil
}

func (ac *VerifyingAssignmentCollector) verifySignature(approval *flow.ResultApproval, nodeIdentity *flow.Identity) error {
	id := approval.Body.ID()
	valid, err := nodeIdentity.StakingPubKey.Verify(approval.VerifierSignature, id[:], ac.sigHasher)
	if err != nil {
		return fmt.Errorf("failed to verify approval signature: %w", err)
	}

	if !valid {
		return engine.NewInvalidInputErrorf("invalid signature for (%x)", nodeIdentity.NodeID)
	}

	return nil
}

// validateApproval performs result level checks of flow.ResultApproval
// checks:
// - verification node identity
// - attestation signature
// - signature of verification node
// - chunk index sanity check
// - block ID sanity check
// Returns:
// - engine.InvalidInputError - result approval is invalid
// - exception in case of any other error, usually this is not expected
// - nil on successful check
func (ac *VerifyingAssignmentCollector) validateApproval(approval *flow.ResultApproval) error {
	// check that approval is for the expected result to reject incompatible inputs
	if approval.Body.ExecutionResultID != ac.ResultID() {
		return fmt.Errorf("AssignmentCollector processes only approvals for result (%x) but got one for (%x)", ac.ResultID(), approval.Body.ExecutionResultID)
	}

	// approval has to refer same block as execution result
	if approval.Body.BlockID != ac.BlockID() {
		return engine.NewInvalidInputErrorf("result approval for invalid block, expected (%x) vs (%x)",
			ac.BlockID(), approval.Body.BlockID)
	}

	chunkIndex := approval.Body.ChunkIndex
	if chunkIndex >= uint64(ac.result.Chunks.Len()) {
		return engine.NewInvalidInputErrorf("chunk index out of range: %v", chunkIndex)
	}

	identity, found := ac.authorizedApprovers[approval.Body.ApproverID]
	if !found {
		return engine.NewInvalidInputErrorf("approval not from authorized verifier")
	}

	err := ac.verifyAttestationSignature(&approval.Body, identity)
	if err != nil {
		return fmt.Errorf("validating attestation signature failed: %w", err)
	}

	err = ac.verifySignature(approval, identity)
	if err != nil {
		return fmt.Errorf("validating approval signature failed: %w", err)
	}

	return nil
}

// ProcessApproval ingests Result Approvals and triggers sealing of execution result
// when sufficient approvals have arrived.
// Error Returns:
//  * nil in case of success (outdated approvals might be silently discarded)
//  * engine.InvalidInputError if the result approval is invalid
//  * any other errors might be symptoms of bugs or internal state corruption (fatal)
func (ac *VerifyingAssignmentCollector) ProcessApproval(approval *flow.ResultApproval) error {
	ac.log.Debug().
		Str("result_id", approval.Body.ExecutionResultID.String()).
		Str("verifier_id", approval.Body.ApproverID.String()).
		Msg("processing result approval")

	// we have this approval cached already, no need to process it again
	// here we need to use PartialID to have a hash over Attestation + ApproverID
	// there is no need to use hash over full approval since it contains extra information
	// and we are only interested in approval body.
	approvalCacheID := approval.Body.PartialID()
	if cached := ac.verifiedApprovalsCache.Get(approvalCacheID); cached != nil {
		return nil
	}

	err := ac.validateApproval(approval)
	if err != nil {
		return fmt.Errorf("could not validate approval: %w", err)
	}

	newlyAdded := ac.verifiedApprovalsCache.Put(approvalCacheID, approval)
	if !newlyAdded {
		return nil
	}

	for _, collector := range ac.allCollectors() {
		// approvals are verified already and shouldn't yield any errors
		err := collector.ProcessApproval(approval)
		if err != nil {
			return fmt.Errorf("could not process approval: %w", err)
		}
	}

	return nil
}

// RequestMissingApprovals traverses all collectors and requests missing approval
// for every chunk that didn't get enough approvals from verifiers.
// Returns number of requests made and error in case something goes wrong.
func (ac *VerifyingAssignmentCollector) RequestMissingApprovals(observation consensus.SealingObservation, maxHeightForRequesting uint64) (uint, error) {
	overallRequestCount := uint(0) // number of approval requests for all different assignments for this result
	for _, collector := range ac.allCollectors() {
		if collector.IncorporatedBlock().Height > maxHeightForRequesting {
			continue
		}

		missingChunks := collector.CollectMissingVerifiers()
		observation.ApprovalsMissing(collector.IncorporatedResult(), missingChunks)
		requestCount := uint(0)
		for chunkIndex, verifiers := range missingChunks {
			// Retrieve information about requests made for this chunk. Skip
			// requesting if the blackout period hasn't expired. Otherwise,
			// update request count and reset blackout period.
			requestTrackerItem, updated, err := ac.requestTracker.TryUpdate(ac.result, collector.IncorporatedBlockID(), chunkIndex)
			if err != nil {
				// it could happen that other gorotuine will prune request tracker because of sealing progress
				// in this case we should just stop requesting approvals as block was already sealed
				if mempool.IsDecreasingPruningHeightError(err) {
					return 0, nil
				}
				return 0, err
			}
			if !updated {
				continue
			}

			// for monitoring/debugging purposes, log requests if we start
			// making more than 10
			if requestTrackerItem.Requests >= 10 {
				log.Debug().Msgf("requesting approvals for result %v, incorporatedBlockID %v chunk %d: %d requests",
					ac.ResultID(),
					collector.IncorporatedBlockID(),
					chunkIndex,
					requestTrackerItem.Requests,
				)
			}

			// prepare the request
			req := &messages.ApprovalRequest{
				Nonce:      rand.Uint64(),
				ResultID:   ac.ResultID(),
				ChunkIndex: chunkIndex,
			}

			requestCount++
			err = ac.approvalConduit.Publish(req, verifiers...)
			if err != nil {
				log.Error().Err(err).
					Msgf("could not publish approval request for chunk %d", chunkIndex)
			}
		}

		observation.ApprovalsRequested(collector.IncorporatedResult(), requestCount)
		overallRequestCount += requestCount
	}

	return overallRequestCount, nil
}

// authorizedVerifiersAtBlock pre-select all authorized Verifiers at the block that incorporates the result.
// The method returns the set of all node IDs that:
//   * are authorized members of the network at the given block and
//   * have the Verification role and
//   * have _positive_ weight and
//   * are not ejected
func authorizedVerifiersAtBlock(state protocol.State, blockID flow.Identifier) (map[flow.Identifier]*flow.Identity, error) {
	authorizedVerifierList, err := state.AtBlockID(blockID).Identities(
		filter.And(
			filter.HasRole(flow.RoleVerification),
			filter.HasWeight(true),
			filter.Not(filter.Ejected),
		))
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve Identities for block %v: %w", blockID, err)
	}
	if len(authorizedVerifierList) == 0 {
		return nil, fmt.Errorf("no authorized verifiers found for block %v", blockID)
	}

	return authorizedVerifierList.Lookup(), nil
}
