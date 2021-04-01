package approvals

import (
	"fmt"
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

	lock sync.RWMutex

	//approvalsCache ApprovalsCache

	// authorized approvers for this collector on execution
	// used to check identity of approver
	authorizedApprovers map[flow.Identifier]flow.Identity

	assigner module.ChunkAssigner
	state    protocol.State
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
func (c *AssignmentCollector) authorizedVerifiersAtBlock(blockID flow.Identifier) (map[flow.Identifier]struct{}, error) {
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
	return authorizedVerifierList.Lookup(), nil
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
	authorizedVerifiers, err := c.authorizedVerifiersAtBlock(incorporatedResult.IncorporatedBlockID)
	if err != nil {
		return fmt.Errorf("could not determine authorized verifiers: %w", err)
	}

	c.lock.Lock()
	defer c.lock.Unlock()
	c.collectors[incorporatedResult.IncorporatedBlockID] = NewApprovalCollector(incorporatedResult, assignment, authorizedVerifiers)
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

func (c *AssignmentCollector) validateApproval(approval *flow.ResultApproval) error {
	// TODO: implement logic that is currently in approval_validator.go
	return nil
}

func (c *AssignmentCollector) ProcessAssignment(approval *flow.ResultApproval) error {
	err := c.validateApproval(approval)
	if err != nil {
		return fmt.Errorf("could not validate approval: %w", err)
	}

	// TODO: add approval into cache before processing.

	for _, collector := range c.allCollectors() {
		err := collector.ProcessApproval(approval)
		if err != nil {
			return fmt.Errorf("could not process assignment for collector %v: %w", collector.incorporatedBlockID, err)
		}
	}
	return nil
}
