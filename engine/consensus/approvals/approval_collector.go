package approvals

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool"
)

// ApprovalCollector is responsible for distributing work to chunk collectors,
// collecting aggregated signatures for chunks that reached seal construction threshold,
// creating and submitting seal candidates once signatures for every chunk are aggregated.
type ApprovalCollector struct {
	chunkCollectors                      []*ChunkApprovalCollector           // slice of chunk collectors that is created on construction and doesn't change
	aggregatedSignatures                 map[uint64]flow.AggregatedSignature // aggregated signature for each chunk
	lock                                 sync.RWMutex                        // lock for modifying aggregatedSignatures
	incorporatedResult                   *flow.IncorporatedResult            // incorporated result that is being sealed
	seals                                mempool.IncorporatedResultSeals     // holds candidate seals for incorporated results that have acquired sufficient approvals; candidate seals are constructed  without consideration of the sealability of parent results
	numberOfChunks                       int                                 // number of chunks for execution result, remains constant
	requiredApprovalsForSealConstruction uint                                // min number of approvals required for constructing a candidate seal
}

func NewApprovalCollector(result *flow.IncorporatedResult, assignment *chunks.Assignment, seals mempool.IncorporatedResultSeals, requiredApprovalsForSealConstruction uint) *ApprovalCollector {
	chunkCollectors := make([]*ChunkApprovalCollector, 0, result.Result.Chunks.Len())
	for _, chunk := range result.Result.Chunks {
		chunkAssignment := assignment.Verifiers(chunk).Lookup()
		collector := NewChunkApprovalCollector(chunkAssignment, requiredApprovalsForSealConstruction)
		chunkCollectors = append(chunkCollectors, collector)
	}
	return &ApprovalCollector{
		incorporatedResult:                   result,
		numberOfChunks:                       result.Result.Chunks.Len(),
		chunkCollectors:                      chunkCollectors,
		requiredApprovalsForSealConstruction: requiredApprovalsForSealConstruction,
		aggregatedSignatures:                 make(map[uint64]flow.AggregatedSignature, result.Result.Chunks.Len()),
		seals:                                seals,
	}
}

func (c *ApprovalCollector) IncorporatedBlockID() flow.Identifier {
	return c.incorporatedResult.IncorporatedBlockID
}

func (c *ApprovalCollector) chunkHasEnoughApprovals(chunkIndex uint64) bool {
	c.lock.RLock()
	defer c.lock.RUnlock()
	_, found := c.aggregatedSignatures[chunkIndex]
	return found
}

func (c *ApprovalCollector) SealResult() error {
	// get final state of execution result
	finalState, ok := c.incorporatedResult.Result.FinalStateCommitment()
	if !ok {
		// message correctness should have been checked before: failure here is an internal implementation bug
		return fmt.Errorf("failed to get final state commitment from Execution Result")
	}

	aggregatedSigs := make([]flow.AggregatedSignature, c.numberOfChunks)

	c.lock.RLock()
	for chunkIndex, sig := range c.aggregatedSignatures {
		aggregatedSigs[chunkIndex] = sig
	}
	c.lock.RUnlock()

	// TODO: Check SPoCK proofs

	// generate & store seal
	seal := &flow.Seal{
		BlockID:                c.incorporatedResult.Result.BlockID,
		ResultID:               c.incorporatedResult.Result.ID(),
		FinalState:             finalState,
		AggregatedApprovalSigs: aggregatedSigs,
	}

	// we don't care if the seal is already in the mempool
	_, err := c.seals.Add(&flow.IncorporatedResultSeal{
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
	if c.chunkHasEnoughApprovals(chunkIndex) {
		return nil
	}

	collector := c.chunkCollectors[chunkIndex]
	aggregatedSignature, collected := collector.ProcessApproval(approval)
	if !collected {
		return nil
	}

	approvedChunks := c.collectAggregatedSignature(chunkIndex, aggregatedSignature)
	if approvedChunks < c.numberOfChunks {
		return nil // still missing approvals for some chunks
	}

	return c.SealResult()
}

// collectAggregatedSignature adds the AggregatedSignature from the collector to `aggregatedSignatures`.
// The returned int is the resulting number of approved chunks.
func (c *ApprovalCollector) collectAggregatedSignature(chunkIndex uint64, aggregatedSignature flow.AggregatedSignature) int {
	c.lock.Lock()
	defer c.lock.Unlock()
	if _, found := c.aggregatedSignatures[chunkIndex]; !found {
		c.aggregatedSignatures[chunkIndex] = aggregatedSignature
	}
	return len(c.aggregatedSignatures)
}

func (c *ApprovalCollector) collectChunksWithMissingApprovals() []uint64 {
	// provide enough capacity to avoid allocations while we hold the lock
	missingChunks := make([]uint64, 0, c.numberOfChunks)
	c.lock.RLock()
	defer c.lock.RUnlock()
	for i := 0; i < c.numberOfChunks; i++ {
		chunkIndex := uint64(i)
		if _, found := c.aggregatedSignatures[chunkIndex]; found {
			// skip if we already have enough valid approvals for this chunk
			continue
		}
		missingChunks = append(missingChunks, chunkIndex)
	}
	return missingChunks
}

// CollectMissingVerifiers collects ids of verifiers who haven't provided an approval for particular chunk
// Returns: map { ChunkIndex -> []VerifierId }
func (c *ApprovalCollector) CollectMissingVerifiers() map[uint64]flow.IdentifierList {
	targetIDs := make(map[uint64]flow.IdentifierList)
	for _, chunkIndex := range c.collectChunksWithMissingApprovals() {
		missingSigners := c.chunkCollectors[chunkIndex].GetMissingSigners()
		if missingSigners.Len() > 0 {
			targetIDs[chunkIndex] = missingSigners
		}
	}

	return targetIDs
}
