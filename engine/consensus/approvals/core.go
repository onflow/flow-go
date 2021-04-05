package approvals

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/state/protocol"
)

// ResultApprovalProcessor performs processing and storing of approvals
type ResultApprovalProcessor interface {
	// submits approval for processing in sync way
	ProcessApproval(approval *flow.ResultApproval) error
	// submits incorporated result for processing in sync way
	ProcessIncorporatedResult(result *flow.IncorporatedResult) error
}

// implementation of ResultApprovalProcessor
type resultApprovalProcessor struct {
	collectors map[flow.Identifier]*AssignmentCollector // contains a mapping of ResultID to AssignmentCollector
	lock       sync.RWMutex                             // lock for collectors
	assigner   module.ChunkAssigner
	state      protocol.State
	verifier   module.Verifier
}

func (p *resultApprovalProcessor) ProcessIncorporatedResult(result *flow.IncorporatedResult) error {
	collector := p.getOrCreateCollector(result.Result.ID())
	err := collector.ProcessIncorporatedResult(result)
	if err != nil {
		return fmt.Errorf("could not process incorporated result: %w", err)
	}
	return nil
}

func (p *resultApprovalProcessor) ProcessApproval(approval *flow.ResultApproval) error {
	collector := p.getOrCreateCollector(approval.Body.ExecutionResultID)
	err := collector.ProcessAssignment(approval)
	if err != nil {
		return fmt.Errorf("could not process assignment: %w", err)
	}
	return nil
}

func (p *resultApprovalProcessor) getCollector(resultID flow.Identifier) *AssignmentCollector {
	p.lock.RLock()
	defer p.lock.RUnlock()
	return p.collectors[resultID]
}

func (p *resultApprovalProcessor) createCollector(resultID flow.Identifier) *AssignmentCollector {
	p.lock.Lock()
	defer p.lock.Unlock()
	collector := NewAssignmentCollector(resultID, p.state, p.assigner, p.verifier)
	p.collectors[resultID] = collector
	return collector
}

func (p *resultApprovalProcessor) getOrCreateCollector(resultID flow.Identifier) *AssignmentCollector {
	if collector := p.getCollector(resultID); collector != nil {
		return collector
	}
	return p.createCollector(resultID)
}
