package approvals

import (
	"fmt"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/state/protocol"
	"sync"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

// AssignmentCollector encapsulates the processing of approvals for one
// specific result
type AssignmentCollector struct {
	resultID flow.Identifier

	// collectors is a mapping IncorporatedBlockID -> ApprovalCollector
	// for every incorporated result we will track approvals separately
	collectors map[flow.Identifier]*ApprovalCollector
	lock       sync.RWMutex // lock for protecting collectors map

	//approvalsCache ApprovalsCache

	// authorized approvers for this collector on execution
	// used to check identity of approver
	authorizedApprovers                  map[flow.Identifier]*flow.Identity
	assigner                             module.ChunkAssigner
	state                                protocol.State
	verifier                             module.Verifier
	requiredApprovalsForSealConstruction uint
}

func NewAssignmentCollector(resultID flow.Identifier, state protocol.State, assigner module.ChunkAssigner,
	sigVerifier module.Verifier, requiredApprovalsForSealConstruction uint) *AssignmentCollector {
	collector := &AssignmentCollector{
		resultID:                             resultID,
		collectors:                           make(map[flow.Identifier]*ApprovalCollector),
		state:                                state,
		assigner:                             assigner,
		verifier:                             sigVerifier,
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
	if collector := c.collectorByBlockID(incorporatedResult.IncorporatedBlockID); collector == nil {
		return nil
	}

	// chunk assigment is based on the first block in the fork that incorporates the result
	assignment, err := c.assigner.Assign(incorporatedResult.Result, incorporatedResult.IncorporatedBlockID)
	if err != nil {
		return fmt.Errorf("could not determine chunk assignment: %w", err)
	}

	// pre-select all authorized Verifiers at the block that incorporates the result
	authorizedVerifiersTmp, err := c.authorizedVerifiersAtBlock(incorporatedResult.IncorporatedBlockID)
	if err != nil {
		return fmt.Errorf("could not determine authorized verifiers: %w", err)
	}

	authorizedVerifiers := make(map[flow.Identifier]struct{})
	for nodeID, _ := range authorizedVerifiersTmp {
		authorizedVerifiers[nodeID] = struct{}{}
	}

	c.authorizedApprovers, err = c.authorizedVerifiersAtBlock(incorporatedResult.Result.BlockID)
	if err != nil {
		return fmt.Errorf("could not determine authorized verifiers for sealing candidate: %w", err)
	}

	c.lock.Lock()
	defer c.lock.Unlock()
	c.collectors[incorporatedResult.IncorporatedBlockID] = NewApprovalCollector(incorporatedResult, assignment,
		authorizedVerifiers, c.requiredApprovalsForSealConstruction)
	return nil
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

func (c *AssignmentCollector) ProcessAssignment(approval *flow.ResultApproval) error {
	err := c.validateApproval(approval)
	if err != nil {
		return fmt.Errorf("could not validate approval: %w", err)
	}

	// TODO: add approval into cache before processing.

	for _, collector := range c.allCollectors() {
		_, err := collector.ProcessApproval(approval)
		if err != nil {
			return fmt.Errorf("could not process assignment for collector %v: %w", collector.incorporatedBlockID, err)
		}
	}
	return nil
}
