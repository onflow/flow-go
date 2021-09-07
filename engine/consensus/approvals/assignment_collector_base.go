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

// AssignmentCollectorBase holds the shared data and functionality for
// implementations of the
// AssignmentCollectorBase holds common dependencies and immutable values that are shared
// by the different states of an AssignmentCollector. It is indented as the base struct
// for the different `AssignmentCollectorState` implementations.
type AssignmentCollectorBase struct {
	log zerolog.Logger

	workerPool                           *workerpool.WorkerPool
	assigner                             module.ChunkAssigner            // component for computing chunk assignments
	state                                protocol.State                  // protocol state
	headers                              storage.Headers                 // used to query headers from storage
	verifier                             module.Verifier                 // used to validate result approvals
	seals                                mempool.IncorporatedResultSeals // holds candidate seals for incorporated results that have acquired sufficient approvals; candidate seals are constructed  without consideration of the sealability of parent results
	approvalConduit                      network.Conduit                 // used to request missing approvals from verification nodes
	requestTracker                       *RequestTracker                 // used to keep track of number of approval requests, and blackout periods, by chunk
	requiredApprovalsForSealConstruction uint                            // number of approvals that are required for each chunk to be sealed

	result        *flow.ExecutionResult // execution result
	resultID      flow.Identifier       // ID of execution result
	executedBlock *flow.Header          // header of the executed block
}

func NewAssignmentCollectorBase(logger zerolog.Logger,
	workerPool *workerpool.WorkerPool,
	result *flow.ExecutionResult,
	state protocol.State,
	headers storage.Headers,
	assigner module.ChunkAssigner,
	seals mempool.IncorporatedResultSeals,
	sigVerifier module.Verifier,
	approvalConduit network.Conduit,
	requestTracker *RequestTracker,
	requiredApprovalsForSealConstruction uint,
) (AssignmentCollectorBase, error) {
	executedBlock, err := headers.ByBlockID(result.BlockID)
	if err != nil {
		return AssignmentCollectorBase{}, err
	}

	return AssignmentCollectorBase{
		log:                                  logger,
		workerPool:                           workerPool,
		assigner:                             assigner,
		state:                                state,
		headers:                              headers,
		verifier:                             sigVerifier,
		seals:                                seals,
		approvalConduit:                      approvalConduit,
		requestTracker:                       requestTracker,
		requiredApprovalsForSealConstruction: requiredApprovalsForSealConstruction,
		result:                               result,
		resultID:                             result.ID(),
		executedBlock:                        executedBlock,
	}, nil
}

func (cb *AssignmentCollectorBase) BlockID() flow.Identifier      { return cb.result.BlockID }
func (cb *AssignmentCollectorBase) Block() *flow.Header           { return cb.executedBlock }
func (cb *AssignmentCollectorBase) ResultID() flow.Identifier     { return cb.resultID }
func (cb *AssignmentCollectorBase) Result() *flow.ExecutionResult { return cb.result }

// OnInvalidApproval logs in invalid approval
func (cb *AssignmentCollectorBase) OnInvalidApproval(approval *flow.ResultApproval, err error) {
	cb.log.Error().Err(err).
		Str("approver_id", approval.Body.ApproverID.String()).
		Str("executed_block_id", approval.Body.BlockID.String()).
		Str("result_id", approval.Body.ExecutionResultID.String()).
		Str("approval_id", approval.ID().String()).
		Msg("received invalid approval")
}
