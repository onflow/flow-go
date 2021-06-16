package approvals

import (
	"fmt"
	"math"
	"math/rand"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/consensus/approvals/tracker"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// DefaultEmergencySealingThreshold is the default number of blocks which indicates that ER should be sealed using emergency
// sealing.
const DefaultEmergencySealingThreshold = 400

// helper functor that can be used to retrieve cached block height
type GetCachedBlockHeight = func(blockID flow.Identifier) (uint64, error)

// AssignmentCollector
// Context:
//  * When the same result is incorporated in multiple different forks,
//    unique verifier assignment is determined for each fork.
//  * The assignment collector is intended to encapsulate the known
//    assignments for a particular execution result.
// AssignmentCollector has a strict ordering of processing, before processing
// approvals at least one incorporated result has to be processed.
// AssignmentCollector takes advantage of internal caching to speed up processing approvals for different assignments
// AssignmentCollector is responsible for validating approvals on result-level (checking signature, identity).
// TODO: currently AssignmentCollector doesn't cleanup collectorTree when blocks that incorporate results get orphaned
// For BFT milestone we need to ensure that this cleanup is properly implemented and all orphan collectorTree are pruned by height
// when fork gets orphaned
type AssignmentCollector struct {
	ResultID                             flow.Identifier                        // ID of execution result
	result                               *flow.ExecutionResult                  // execution result that we are collecting approvals for
	BlockHeight                          uint64                                 // height of block targeted by execution result
	collectors                           map[flow.Identifier]*ApprovalCollector // collectors is a mapping IncorporatedBlockID -> ApprovalCollector
	authorizedApprovers                  map[flow.Identifier]*flow.Identity     // map of approvers pre-selected at block that is being sealed
	lock                                 sync.RWMutex                           // lock for protecting collectors map
	verifiedApprovalsCache               *Cache                                 // in-memory cache of approvals (already verified)
	requiredApprovalsForSealConstruction uint                                   // number of approvals that are required for each chunk to be sealed
	assigner                             module.ChunkAssigner                   // used to build assignment
	headers                              storage.Headers                        // used to query headers from storage
	state                                protocol.State                         // used to access the  protocol state
	verifier                             module.Verifier                        // used to validate result approvals
	seals                                mempool.IncorporatedResultSeals        // holds candidate seals for incorporated results that have acquired sufficient approvals; candidate seals are constructed  without consideration of the sealability of parent results
	approvalConduit                      network.Conduit                        // used to request missing approvals from verification nodes
	requestTracker                       *RequestTracker                        // used to keep track of number of approval requests, and blackout periods, by chunk
}

func NewAssignmentCollector(result *flow.ExecutionResult, state protocol.State, headers storage.Headers, assigner module.ChunkAssigner, seals mempool.IncorporatedResultSeals,
	sigVerifier module.Verifier, approvalConduit network.Conduit, requestTracker *RequestTracker, requiredApprovalsForSealConstruction uint,
) (*AssignmentCollector, error) {
	block, err := headers.ByBlockID(result.BlockID)
	if err != nil {
		return nil, err
	}
	// pre-select all authorized verifiers at the block that is being sealed
	authorizedApprovers, err := authorizedVerifiersAtBlock(state, result.BlockID)
	if err != nil {
		return nil, engine.NewInvalidInputErrorf("could not determine authorized verifiers for sealing candidate: %w", err)
	}

	collector := &AssignmentCollector{
		ResultID:                             result.ID(),
		result:                               result,
		BlockHeight:                          block.Height,
		collectors:                           make(map[flow.Identifier]*ApprovalCollector),
		authorizedApprovers:                  authorizedApprovers,
		verifiedApprovalsCache:               NewApprovalsCache(uint(result.Chunks.Len() * len(authorizedApprovers))),
		state:                                state,
		assigner:                             assigner,
		seals:                                seals,
		verifier:                             sigVerifier,
		requestTracker:                       requestTracker,
		approvalConduit:                      approvalConduit,
		headers:                              headers,
		requiredApprovalsForSealConstruction: requiredApprovalsForSealConstruction,
	}
	return collector, nil
}

// BlockID returns the ID of the executed block
func (ac *AssignmentCollector) BlockID() flow.Identifier {
	return ac.result.BlockID
}

func (ac *AssignmentCollector) collectorByBlockID(incorporatedBlockID flow.Identifier) *ApprovalCollector {
	ac.lock.RLock()
	defer ac.lock.RUnlock()
	return ac.collectors[incorporatedBlockID]
}

// emergencySealable determines whether an incorporated Result qualifies for "emergency sealing".
// ATTENTION: this is a temporary solution, which is NOT BFT compatible. When the approval process
// hangs far enough behind finalization (measured in finalized but unsealed blocks), emergency
// sealing kicks in. This will be removed when implementation of Sealing & Verification is finished.
func (ac *AssignmentCollector) emergencySealable(collector *ApprovalCollector, finalizedBlockHeight uint64) bool {
	// Criterion for emergency sealing:
	// there must be at least DefaultEmergencySealingThreshold number of blocks between
	// the block that _incorporates_ result and the latest finalized block
	return collector.IncorporatedBlock().Height+DefaultEmergencySealingThreshold <= finalizedBlockHeight
}

func (ac *AssignmentCollector) CheckEmergencySealing(finalizedBlockHeight uint64) error {
	for _, collector := range ac.allCollectors() {
		sealable := ac.emergencySealable(collector, finalizedBlockHeight)
		if sealable {
			err := collector.SealResult()
			if err != nil {
				return fmt.Errorf("could not create emergency seal for result %x incorporated at %x: %w",
					ac.ResultID, collector.IncorporatedBlockID(), err)
			}
		}
	}

	return nil
}

func (ac *AssignmentCollector) ProcessIncorporatedResult(incorporatedResult *flow.IncorporatedResult) error {
	// check that result is the one that this AssignmentCollector manages
	if irID := incorporatedResult.Result.ID(); irID != ac.ResultID {
		return fmt.Errorf("this AssignmentCollector manages result %x but got %x", ac.ResultID, irID)
	}

	incorporatedBlockID := incorporatedResult.IncorporatedBlockID
	if collector := ac.collectorByBlockID(incorporatedBlockID); collector != nil {
		return nil
	}

	// Constructing ApprovalCollector for IncorporatedResult
	// The AssignmentCollector is not locked while instantiating the ApprovalCollector. Hence, it is possible that
	// multiple threads simultaneously compute the verifier assignment. Nevertheless, the implementation is safe in
	// that only one of the instantiated ApprovalCollectors will be stored in the cache. In terms of locking duration,
	// it's better to perform extra computation in edge cases than lock this logic with a mutex,
	// since it's quite unlikely that same incorporated result will be processed by multiple goroutines simultaneously
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
	collector := NewApprovalCollector(incorporatedResult, incorporatedBlock, executedBlock, assignment, ac.seals, ac.requiredApprovalsForSealConstruction)

	// Now, we add the ApprovalCollector to the AssignmentCollector:
	// no-op if an ApprovalCollector has already been added by a different routine
	isDuplicate := ac.putCollector(incorporatedBlockID, collector)
	if isDuplicate {
		return nil
	}

	// process approvals that have passed needed checks and are ready to be processed
	for _, approval := range ac.verifiedApprovalsCache.All() {
		// those approvals are verified already and shouldn't yield any errors
		_ = collector.ProcessApproval(approval)
	}

	return nil
}

func (ac *AssignmentCollector) putCollector(incorporatedBlockID flow.Identifier, collector *ApprovalCollector) bool {
	ac.lock.Lock()
	defer ac.lock.Unlock()
	if _, ok := ac.collectors[incorporatedBlockID]; ok {
		return true
	}
	ac.collectors[incorporatedBlockID] = collector
	return false
}

func (ac *AssignmentCollector) allCollectors() []*ApprovalCollector {
	ac.lock.RLock()
	defer ac.lock.RUnlock()
	collectors := make([]*ApprovalCollector, 0, len(ac.collectors))
	for _, collector := range ac.collectors {
		collectors = append(collectors, collector)
	}
	return collectors
}

func (ac *AssignmentCollector) verifyAttestationSignature(approval *flow.ResultApprovalBody, nodeIdentity *flow.Identity) error {
	id := approval.Attestation.ID()
	valid, err := ac.verifier.Verify(id[:], approval.AttestationSignature, nodeIdentity.StakingPubKey)
	if err != nil {
		return fmt.Errorf("failed to verify attestation signature: %w", err)
	}

	if !valid {
		return engine.NewInvalidInputErrorf("invalid attestation signature for (%x)", nodeIdentity.NodeID)
	}

	return nil
}

func (ac *AssignmentCollector) verifySignature(approval *flow.ResultApproval, nodeIdentity *flow.Identity) error {
	id := approval.Body.ID()
	valid, err := ac.verifier.Verify(id[:], approval.VerifierSignature, nodeIdentity.StakingPubKey)
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
func (ac *AssignmentCollector) validateApproval(approval *flow.ResultApproval) error {
	// check that approval is for the expected result to reject incompatible inputs
	if approval.Body.ExecutionResultID != ac.ResultID {
		return fmt.Errorf("this AssignmentCollector processes only approvals for result (%x) but got an approval for (%x)", ac.ResultID, approval.Body.ExecutionResultID)
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

func (ac *AssignmentCollector) ProcessApproval(approval *flow.ResultApproval) error {
	// we have this approval cached already, no need to process it again
	approvalCacheID := approval.Body.PartialID()
	if cached := ac.verifiedApprovalsCache.Get(approvalCacheID); cached != nil {
		return nil
	}

	err := ac.validateApproval(approval)
	if err != nil {
		return fmt.Errorf("could not validate approval: %w", err)
	}

	newlyAdded := ac.verifiedApprovalsCache.Put(approval)
	if !newlyAdded {
		return nil
	}

	for _, collector := range ac.allCollectors() {
		err := collector.ProcessApproval(approval)
		if err != nil {
			return fmt.Errorf("could not process approval: %w", err)
		}
	}

	return nil
}

// RequestMissingApprovals traverses all collectors and requests missing approval for every chunk that didn't get enough
// approvals from verifiers.
// Returns number of requests made and error in case something goes wrong.
func (ac *AssignmentCollector) RequestMissingApprovals(sealingTracker *tracker.SealingTracker, maxHeightForRequesting uint64) (int, error) {
	requestCount := 0
	for _, collector := range ac.allCollectors() {
		if collector.IncorporatedBlock().Height > maxHeightForRequesting {
			continue
		}

		firstChunkWithMissingApproval := uint64(math.MaxUint64)
		missingChunks := collector.CollectMissingVerifiers()
		for chunkIndex, verifiers := range missingChunks {
			if firstChunkWithMissingApproval < chunkIndex {
				firstChunkWithMissingApproval = chunkIndex
			}

			// Retrieve information about requests made for this chunk. Skip
			// requesting if the blackout period hasn't expired. Otherwise,
			// update request count and reset blackout period.
			requestTrackerItem := ac.requestTracker.Get(ac.ResultID, collector.IncorporatedBlockID(), chunkIndex)
			if requestTrackerItem.IsBlackout() {
				continue
			}
			requestTrackerItem.Update()
			ac.requestTracker.Set(ac.ResultID, collector.IncorporatedBlockID(), chunkIndex, requestTrackerItem)

			// for monitoring/debugging purposes, log requests if we start
			// making more than 10
			if requestTrackerItem.Requests >= 10 {
				log.Debug().Msgf("requesting approvals for result %v, incorporatedBlockID %v chunk %d: %d requests",
					ac.ResultID,
					collector.IncorporatedBlockID(),
					chunkIndex,
					requestTrackerItem.Requests,
				)
			}

			// prepare the request
			req := &messages.ApprovalRequest{
				Nonce:      rand.Uint64(),
				ResultID:   ac.ResultID,
				ChunkIndex: chunkIndex,
			}

			requestCount++
			err := ac.approvalConduit.Publish(req, verifiers...)
			if err != nil {
				log.Error().Err(err).
					Msgf("could not publish approval request for chunk %d", chunkIndex)
			}
		}

		if sealingTracker != nil && len(missingChunks) > 0 {
			sealingRecord := tracker.NewRecordWithInsufficientApprovals(collector.incorporatedResult, firstChunkWithMissingApproval)
			sealingTracker.Track(sealingRecord)
		}

	}
	return requestCount, nil
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
			filter.HasStake(true),
			filter.Not(filter.Ejected),
		))
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve Identities for block %v: %w", blockID, err)
	}
	if len(authorizedVerifierList) == 0 {
		return nil, fmt.Errorf("no authorized verifiers found for block %v", blockID)
	}
	identities := make(map[flow.Identifier]*flow.Identity, len(authorizedVerifierList))
	for _, identity := range authorizedVerifierList {
		identities[identity.NodeID] = identity
	}
	return identities, nil
}
