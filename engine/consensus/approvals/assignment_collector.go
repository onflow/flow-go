package approvals

import (
	"fmt"
	"math/rand"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/consensus/sealing"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/state/protocol"
)

// AssignmentCollector is responsible collecting approvals that satisfy one assignment, meaning that we will
// have multiple collectors for one execution result as same result can be incorporated in multiple forks.
// AssignmentCollector has a strict ordering of processing, before processing approvals at least one incorporated result has to be
// processed.
// AssignmentCollector takes advantage of internal caching to speed up processing approvals for different assignments
// AssignmentCollector is responsible for validating approvals on result-level(checking signature, identity).
type AssignmentCollector struct {
	ResultID                             flow.Identifier
	collectors                           map[flow.Identifier]*ApprovalCollector // collectors is a mapping IncorporatedBlockID -> ApprovalCollector
	authorizedApprovers                  map[flow.Identifier]*flow.Identity     // map of approvers pre-selected at block that is being sealed
	lock                                 sync.RWMutex                           // lock for protecting collectors map
	verifiedApprovalsCache               *ApprovalsCache                        // in-memory cache of approvals were already verified
	requiredApprovalsForSealConstruction uint                                   // number of approvals that are required for each chunk to be sealed

	assigner        module.ChunkAssigner
	state           protocol.State
	verifier        module.Verifier
	seals           mempool.IncorporatedResultSeals
	approvalConduit network.Conduit         // used to request missing approvals from verification nodes
	requestTracker  *sealing.RequestTracker // used to keep track of number of approval requests, and blackout periods, by chunk
}

func NewAssignmentCollector(resultID flow.Identifier, state protocol.State, assigner module.ChunkAssigner, seals mempool.IncorporatedResultSeals,
	sigVerifier module.Verifier, approvalConduit network.Conduit, requestTracker *sealing.RequestTracker, requiredApprovalsForSealConstruction uint) *AssignmentCollector {
	collector := &AssignmentCollector{
		verifiedApprovalsCache:               NewApprovalsCache(1000),
		ResultID:                             resultID,
		collectors:                           make(map[flow.Identifier]*ApprovalCollector),
		state:                                state,
		assigner:                             assigner,
		seals:                                seals,
		verifier:                             sigVerifier,
		requestTracker:                       requestTracker,
		approvalConduit:                      approvalConduit,
		requiredApprovalsForSealConstruction: requiredApprovalsForSealConstruction,
	}
	return collector
}

func (c *AssignmentCollector) collectorByBlockID(incorporatedBlockID flow.Identifier) *ApprovalCollector {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.collectors[incorporatedBlockID]
}

// authorizedVerifiersAtBlock pre-select all authorized Verifiers at the block that incorporates the result.
// The method returns the set of all node IDs that:
//   * are authorized members of the network at the given block and
//   * have the Verification role and
//   * have _positive_ weight and
//   * are not ejected
func (c *AssignmentCollector) authorizedVerifiersAtBlock(blockID flow.Identifier) (map[flow.Identifier]*flow.Identity, error) {
	authorizedVerifierList, err := c.state.AtBlockID(blockID).Identities(
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
	identities := make(map[flow.Identifier]*flow.Identity)
	for _, identity := range authorizedVerifierList.Copy() {
		identities[identity.NodeID] = identity
	}
	return identities, nil
}

func (c *AssignmentCollector) ProcessIncorporatedResult(incorporatedResult *flow.IncorporatedResult) error {
	if collector := c.collectorByBlockID(incorporatedResult.IncorporatedBlockID); collector != nil {
		return nil
	}

	// chunk assigment is based on the first block in the fork that incorporates the result
	assignment, err := c.assigner.Assign(incorporatedResult.Result, incorporatedResult.IncorporatedBlockID)
	if err != nil {
		return engine.NewInvalidInputErrorf("could not determine chunk assignment: %w", err)
	}

	// pre-select all authorized verifiers at the block that is being sealed
	c.authorizedApprovers, err = c.authorizedVerifiersAtBlock(incorporatedResult.Result.BlockID)
	if err != nil {
		return engine.NewInvalidInputErrorf("could not determine authorized verifiers for sealing candidate: %w", err)
	}

	collector := NewApprovalCollector(incorporatedResult, assignment, c.seals, c.requiredApprovalsForSealConstruction)

	c.putCollector(incorporatedResult.IncorporatedBlockID, collector)

	// process approvals that have passed needed checks and are ready to be processed
	for _, approvalID := range c.verifiedApprovalsCache.Ids() {
		if approval := c.verifiedApprovalsCache.Peek(approvalID); approval != nil {
			// those approvals are verified already and shouldn't yield any errors
			_ = collector.ProcessApproval(approval)
		}
	}

	return nil
}

func (c *AssignmentCollector) putCollector(incorporatedBlockID flow.Identifier, collector *ApprovalCollector) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.collectors[incorporatedBlockID] = collector
}

func (c *AssignmentCollector) allCollectors() []*ApprovalCollector {
	c.lock.RLock()
	defer c.lock.RUnlock()
	collectors := make([]*ApprovalCollector, 0, len(c.collectors))
	for _, collector := range c.collectors {
		collectors = append(collectors, collector)
	}
	return collectors
}

func (c *AssignmentCollector) verifySignature(approval *flow.ResultApproval, nodeIdentity *flow.Identity) error {
	id := approval.Body.ID()
	valid, err := c.verifier.Verify(id[:], approval.VerifierSignature, nodeIdentity.StakingPubKey)
	if err != nil {
		return fmt.Errorf("failed to verify signature: %w", err)
	}

	if !valid {
		return engine.NewInvalidInputErrorf("invalid signature for (%x)", nodeIdentity.NodeID)
	}

	return nil
}

// validateApproval performs result level checks of flow.ResultApproval
// checks:
// 	verification node identity
//  signature of verification node
// returns nil on successful check
func (c *AssignmentCollector) validateApproval(approval *flow.ResultApproval) error {
	identity, found := c.authorizedApprovers[approval.Body.ApproverID]
	if !found {
		return engine.NewInvalidInputErrorf("approval not from authorized verifier")
	}

	err := c.verifySignature(approval, identity)
	if err != nil {
		return fmt.Errorf("invalid approval signature: %w", err)
	}

	return nil
}

// validateAndCache performs validation of approval and saves it into cache
// expects that execution result was discovered before calling this function
func (c *AssignmentCollector) validateAndCache(approval *flow.ResultApproval) error {
	err := c.validateApproval(approval)
	if err != nil {
		return fmt.Errorf("could not validate approval: %w", err)
	}

	c.verifiedApprovalsCache.Put(approval)
	return nil
}

func (c *AssignmentCollector) ProcessAssignment(approval *flow.ResultApproval) error {
	err := c.validateAndCache(approval)
	if err != nil {
		return fmt.Errorf("could not validate and cache approval: %w", err)
	}

	for _, collector := range c.allCollectors() {
		err := collector.ProcessApproval(approval)
		if err != nil {
			return fmt.Errorf("could not process assignment for collector %v: %w", collector.incorporatedResult.IncorporatedBlockID, err)
		}
	}

	return nil
}

func (c *AssignmentCollector) RequestMissingApprovals(maxHeightForRequesting uint64) error {
	for _, collector := range c.allCollectors() {
		// not finding the block that the result was incorporated in is a fatal
		// error at this stage
		block, err := c.state.AtBlockID(collector.IncorporatedBlockID).Head()
		if err != nil {
			return fmt.Errorf("could not retrieve block: %w", err)
		}

		if block.Height > maxHeightForRequesting {
			continue
		}

		// If we got this far, height `block.Height` must be finalized, because
		// maxHeightForRequesting is lower than the finalized height.

		// Skip result if it is incorporated in a block that is _not_ part of
		// the finalized fork.
		//finalizedBlockAtHeight, err := c.state.AtHeight(block.Height).Head()
		//if err != nil {
		//	return fmt.Errorf("could not retrieve finalized block for finalized height %d: %w", block.Height, err)
		//}
		// TODO: replace this check with cleanup while processing finalized blocks in approval processing core
		//if finalizedBlockAtHeight.ID() != collector.IncorporatedBlockID {
		//	// block is in an orphaned fork
		//	continue
		//}

		for chunkIndex, verifiers := range collector.CollectMissingVerifiers() {
			// Retrieve information about requests made for this chunk. Skip
			// requesting if the blackout period hasn't expired. Otherwise,
			// update request count and reset blackout period.
			requestTrackerItem := c.requestTracker.Get(c.ResultID, collector.IncorporatedBlockID, chunkIndex)
			if requestTrackerItem.IsBlackout() {
				continue
			}
			requestTrackerItem.Update()

			// for monitoring/debugging purposes, log requests if we start
			// making more than 10
			if requestTrackerItem.Requests >= 10 {
				log.Debug().Msgf("requesting approvals for result %v, incorporatedBlockID %v chunk %d: %d requests",
					c.ResultID,
					collector.IncorporatedBlockID,
					chunkIndex,
					requestTrackerItem.Requests,
				)
			}

			// prepare the request
			req := &messages.ApprovalRequest{
				Nonce:      rand.Uint64(),
				ResultID:   c.ResultID,
				ChunkIndex: chunkIndex,
			}

			err = c.approvalConduit.Publish(req, verifiers...)
			if err != nil {
				log.Error().Err(err).
					Msgf("could not publish approval request for chunk %d", chunkIndex)
			}
		}
	}
	return nil
}
