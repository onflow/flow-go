package approvals

import (
	"github.com/gammazero/workerpool"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type base struct {
	log zerolog.Logger

	workerpool                           *workerpool.WorkerPool
	assigner                             module.ChunkAssigner            // component for computing chunk assignments
	state                                protocol.State                  // protocol state
	headers                              storage.Headers                 // used to query headers from storage
	verifier                             module.Verifier                 // used to validate result approvals
	seals                                mempool.IncorporatedResultSeals // holds candidate seals for incorporated results that have acquired sufficient approvals; candidate seals are constructed  without consideration of the sealability of parent results
	approvalConduit                      network.Conduit                 // used to request missing approvals from verification nodes
	requestTracker                       *RequestTracker                 // used to keep track of number of approval requests, and blackout periods, by chunk
	requiredApprovalsForSealConstruction uint                            // number of approvals that are required for each chunk to be sealed
}

type assignmentCollectorBase struct {
	base
	result        *flow.ExecutionResult // execution result
	resultID      flow.Identifier       // ID of execution result
	executedBlock *flow.Header          // header of the executed block
}

func NewAssignmentCollectorBase(result *flow.ExecutionResult, base base) (assignmentCollectorBase, error) {
	executedBlock, err := base.headers.ByBlockID(result.BlockID)
	if err != nil {
		return assignmentCollectorBase{}, err
	}

	return assignmentCollectorBase{
		base:          base,
		result:        result,
		resultID:      result.ID(),
		executedBlock: executedBlock,
	}, nil
}

func (cb *assignmentCollectorBase) BlockID() flow.Identifier      { return cb.result.BlockID }
func (cb *assignmentCollectorBase) Block() *flow.Header           { return cb.executedBlock }
func (cb *assignmentCollectorBase) ResultID() flow.Identifier     { return cb.resultID }
func (cb *assignmentCollectorBase) Result() *flow.ExecutionResult { return cb.result }

// OnInvalidApproval logs in invalid approval
func (cb *assignmentCollectorBase) OnInvalidApproval(approval *flow.ResultApproval, err error) {
	cb.log.Error().Err(err).
		Str("approver_id", approval.Body.ApproverID.String()).
		Str("executed_block_id", approval.Body.BlockID.String()).
		Str("result_id", approval.Body.ExecutionResultID.String()).
		Str("approval_id", approval.ID().String()).
		Msg("received invalid approval")
}
