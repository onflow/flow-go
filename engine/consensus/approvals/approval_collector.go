package approvals

import (
	"fmt"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool"
)

// ApprovalCollector is responsible for distributing work to chunk collectorTree,
// collecting aggregated signatures for chunks that reached seal construction threshold,
// creating and submitting seal candidates once signatures for every chunk are aggregated.
type ApprovalCollector struct {
	incorporatedBlock                    *flow.Header                    // block that incorporates execution result
	incorporatedResult                   *flow.IncorporatedResult        // incorporated result that is being sealed
	chunkCollectors                      []*ChunkApprovalCollector       // slice of chunk collectorTree that is created on construction and doesn't change
	aggregatedSignatures                 *AggregatedSignatures           // aggregated signature for each chunk
	seals                                mempool.IncorporatedResultSeals // holds candidate seals for incorporated results that have acquired sufficient approvals; candidate seals are constructed  without consideration of the sealability of parent results
	numberOfChunks                       int                             // number of chunks for execution result, remains constant
	requiredApprovalsForSealConstruction uint                            // min number of approvals required for constructing a candidate seal
}

func NewApprovalCollector(result *flow.IncorporatedResult, incorporatedBlock *flow.Header, assignment *chunks.Assignment, seals mempool.IncorporatedResultSeals, requiredApprovalsForSealConstruction uint) *ApprovalCollector {
	chunkCollectors := make([]*ChunkApprovalCollector, 0, result.Result.Chunks.Len())
	for _, chunk := range result.Result.Chunks {
		chunkAssignment := assignment.Verifiers(chunk).Lookup()
		collector := NewChunkApprovalCollector(chunkAssignment, requiredApprovalsForSealConstruction)
		chunkCollectors = append(chunkCollectors, collector)
	}

	numberOfChunks := result.Result.Chunks.Len()
	return &ApprovalCollector{
		incorporatedResult:                   result,
		incorporatedBlock:                    incorporatedBlock,
		numberOfChunks:                       numberOfChunks,
		chunkCollectors:                      chunkCollectors,
		requiredApprovalsForSealConstruction: requiredApprovalsForSealConstruction,
		aggregatedSignatures:                 NewAggregatedSignatures(uint64(numberOfChunks)),
		seals:                                seals,
	}
}

// IncorporatedBlockID returns the ID of block which incorporates execution result
func (c *ApprovalCollector) IncorporatedBlockID() flow.Identifier {
	return c.incorporatedResult.IncorporatedBlockID
}

// IncorporatedBlock returns the block which incorporates execution result
func (c *ApprovalCollector) IncorporatedBlock() *flow.Header {
	return c.incorporatedBlock
}

func (c *ApprovalCollector) SealResult() error {
	// get final state of execution result
	finalState, err := c.incorporatedResult.Result.FinalStateCommitment()
	if err != nil {
		// message correctness should have been checked before: failure here is an internal implementation bug
		return fmt.Errorf("failed to get final state commitment from Execution Result: %w", err)
	}

	// TODO: Check SPoCK proofs

	// generate & store seal
	seal := &flow.Seal{
		BlockID:                c.incorporatedResult.Result.BlockID,
		ResultID:               c.incorporatedResult.Result.ID(),
		FinalState:             finalState,
		AggregatedApprovalSigs: c.aggregatedSignatures.Collect(),
	}

	// we don't care if the seal is already in the mempool
	_, err = c.seals.Add(&flow.IncorporatedResultSeal{
		IncorporatedResult: c.incorporatedResult,
		Seal:               seal,
	})
	if err != nil {
		return fmt.Errorf("failed to store IncorporatedResultSeal in mempool: %w", err)
	}

	return nil
}

// ProcessApproval performs processing of result approvals and bookkeeping of aggregated signatures
// for every chunk. Triggers sealing of execution result when processed last result approval needed for sealing.
// Returns:
// - engine.InvalidInputError - result approval is invalid
// - exception in case of any other error, usually this is not expected
// - nil on success
func (c *ApprovalCollector) ProcessApproval(approval *flow.ResultApproval) error {
	chunkIndex := approval.Body.ChunkIndex
	if chunkIndex >= uint64(len(c.chunkCollectors)) {
		return engine.NewInvalidInputErrorf("approval collector chunk index out of range: %v", chunkIndex)
	}
	// there is no need to process approval if we have already enough info for sealing
	if c.aggregatedSignatures.HasSignature(chunkIndex) {
		return nil
	}

	collector := c.chunkCollectors[chunkIndex]
	aggregatedSignature, collected := collector.ProcessApproval(approval)
	if !collected {
		return nil
	}

	approvedChunks := c.aggregatedSignatures.PutSignature(chunkIndex, aggregatedSignature)
	if approvedChunks < c.numberOfChunks {
		return nil // still missing approvals for some chunks
	}

	return c.SealResult()
}

// CollectMissingVerifiers collects ids of verifiers who haven't provided an approval for particular chunk
// Returns: map { ChunkIndex -> []VerifierId }
func (c *ApprovalCollector) CollectMissingVerifiers() map[uint64]flow.IdentifierList {
	targetIDs := make(map[uint64]flow.IdentifierList)
	for _, chunkIndex := range c.aggregatedSignatures.CollectChunksWithMissingApprovals() {
		missingSigners := c.chunkCollectors[chunkIndex].GetMissingSigners()
		if missingSigners.Len() > 0 {
			targetIDs[chunkIndex] = missingSigners
		}
	}

	return targetIDs
}
