package approvals

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool"
)

// ApprovalCollector is responsible for distributing work to chunk collectorTree,
// collecting aggregated signatures for chunks that reached seal construction threshold,
// creating and submitting seal candidates once signatures for every chunk are aggregated.
type ApprovalCollector struct {
	log                  zerolog.Logger
	incorporatedBlock    *flow.Header                    // block that incorporates execution result
	executedBlock        *flow.Header                    // block that was executed
	incorporatedResult   *flow.IncorporatedResult        // incorporated result that is being sealed
	chunkCollectors      []*ChunkApprovalCollector       // slice of chunk collectorTree that is created on construction and doesn't change
	aggregatedSignatures *AggregatedSignatures           // aggregated signature for each chunk
	seals                mempool.IncorporatedResultSeals // holds candidate seals for incorporated results that have acquired sufficient approvals; candidate seals are constructed  without consideration of the sealability of parent results
	numberOfChunks       uint64                          // number of chunks for execution result, remains constant
}

func NewApprovalCollector(
	log zerolog.Logger,
	result *flow.IncorporatedResult,
	incorporatedBlock *flow.Header,
	executedBlock *flow.Header,
	assignment *chunks.Assignment,
	seals mempool.IncorporatedResultSeals,
	requiredApprovalsForSealConstruction uint,
) (*ApprovalCollector, error) {
	chunkCollectors := make([]*ChunkApprovalCollector, 0, result.Result.Chunks.Len())
	for _, chunk := range result.Result.Chunks {
		chunkAssignment := assignment.Verifiers(chunk).Lookup()
		collector := NewChunkApprovalCollector(chunkAssignment, requiredApprovalsForSealConstruction)
		chunkCollectors = append(chunkCollectors, collector)
	}

	numberOfChunks := uint64(result.Result.Chunks.Len())
	aggSigs, err := NewAggregatedSignatures(numberOfChunks)
	if err != nil {
		return nil, fmt.Errorf("instantiation of AggregatedSignatures failed: %w", err)
	}
	collector := ApprovalCollector{
		log: log.With().
			Str("component", "approval_collector").
			Str("incorporated_block", incorporatedBlock.ID().String()).
			Str("executed_block", executedBlock.ID().String()).
			Logger(),
		incorporatedResult:   result,
		incorporatedBlock:    incorporatedBlock,
		executedBlock:        executedBlock,
		numberOfChunks:       numberOfChunks,
		chunkCollectors:      chunkCollectors,
		aggregatedSignatures: aggSigs,
		seals:                seals,
	}

	// The following code implements a TEMPORARY SHORTCUT: In case no approvals are required
	// to seal an incorporated result, we seal right away when creating the ApprovalCollector.
	if requiredApprovalsForSealConstruction == 0 {
		// The high-level logic is: as soon as we have collected enough approvals, we aggregate
		// them and store them in collector.aggregatedSignatures. If we don't require any signatures,
		// this condition is satisfied right away. Hence, we add aggregated signature for each chunk.
		for i := uint64(0); i < numberOfChunks; i++ {
			_, err := collector.aggregatedSignatures.PutSignature(i, flow.AggregatedSignature{})
			if err != nil {
				return nil, fmt.Errorf("sealing result %x failed: %w", result.ID(), err)
			}
		}
		err := collector.SealResult()
		if err != nil {
			return nil, fmt.Errorf("sealing result %x failed: %w", result.ID(), err)
		}
	}

	return &collector, nil
}

// IncorporatedBlockID returns the ID of block which incorporates execution result
func (c *ApprovalCollector) IncorporatedBlockID() flow.Identifier {
	return c.incorporatedResult.IncorporatedBlockID
}

// IncorporatedBlock returns the block which incorporates execution result
func (c *ApprovalCollector) IncorporatedBlock() *flow.Header {
	return c.incorporatedBlock
}

// IncorporatedResult returns the incorporated Result this ApprovalCollector is for
func (c *ApprovalCollector) IncorporatedResult() *flow.IncorporatedResult {
	return c.incorporatedResult
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

	// Adding a seal that already exists in the mempool is a NoOp. But to reduce log
	// congestion, we only log when a seal is added that previously did not exist.
	added, err := c.seals.Add(&flow.IncorporatedResultSeal{
		IncorporatedResult: c.incorporatedResult,
		Seal:               seal,
		Header:             c.executedBlock,
	})
	if err != nil {
		return fmt.Errorf("failed to store IncorporatedResultSeal in mempool: %w", err)
	}
	if added {
		c.log.Info().
			Str("executed_block_id", seal.BlockID.String()).
			Uint64("executed_block_height", c.executedBlock.Height).
			Str("result_id", seal.ResultID.String()).
			Str("incorporating_block", c.IncorporatedBlockID().String()).
			Msg("added candidate seal to IncorporatedResultSeals mempool")
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
	c.log.Debug().
		Str("result_id", approval.Body.ExecutionResultID.String()).
		Str("verifier_id", approval.Body.ApproverID.String()).
		Msg("processing result approval")

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

	approvedChunks, err := c.aggregatedSignatures.PutSignature(chunkIndex, aggregatedSignature)
	if err != nil {
		return fmt.Errorf("adding aggregated signature failed: %w", err)
	}
	if approvedChunks < c.numberOfChunks {
		return nil // still missing approvals for some chunks
	}

	return c.SealResult()
}

// CollectMissingVerifiers collects ids of verifiers who haven't provided an approval for particular chunk
// Returns: map { ChunkIndex -> []VerifierId }
func (c *ApprovalCollector) CollectMissingVerifiers() map[uint64]flow.IdentifierList {
	targetIDs := make(map[uint64]flow.IdentifierList)
	for _, chunkIndex := range c.aggregatedSignatures.ChunksWithoutAggregatedSignature() {
		missingSigners := c.chunkCollectors[chunkIndex].GetMissingSigners()
		if missingSigners.Len() > 0 {
			targetIDs[chunkIndex] = missingSigners
		}
	}

	return targetIDs
}
