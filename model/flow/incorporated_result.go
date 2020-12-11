package flow

import (
	"sync"

	"github.com/onflow/flow-go/crypto"
)

// IncorporatedResult is a wrapper around an ExecutionResult which contains the
// ID of the first block on its fork in which it was incorporated.
type IncorporatedResult struct {
	// IncorporatedBlockID is the ID of the first block on its fork where a
	// receipt for this result was incorporated. Within a fork, multiple blocks
	// may contain receipts for the same result; only the first one is used to
	// compute the random beacon of the result's chunk assignment.
	IncorporatedBlockID Identifier

	// Result is the ExecutionResult contained in the ExecutionReceipt that was
	// incorporated in the payload of IncorporatedBlockID.
	Result *ExecutionResult

	// aggregatedSignatures is a placeholder for attestation signatures
	// collected for each chunk. It gets populated by the consensus matching
	// engine when approvals are matched to execution results.
	// This field is not exported (name doesn't start with a capital letter), so
	// it is not used in calculating the ID and Checksum of the Incorporated
	// Result (RLP encoding ignores private fields).
	aggregatedSignatures     map[uint64]*AggregatedSignature
	aggregatedSignaturesLock sync.Mutex
}

func NewIncorporatedResult(incorporatedBlockID Identifier, result *ExecutionResult) *IncorporatedResult {
	return &IncorporatedResult{
		IncorporatedBlockID:  incorporatedBlockID,
		Result:               result,
		aggregatedSignatures: make(map[uint64]*AggregatedSignature),
	}
}

// ID implements flow.Entity.ID for IncorporatedResult to make it capable of
// being stored directly in mempools and storage.
func (ir *IncorporatedResult) ID() Identifier {
	return MakeID([2]Identifier{ir.IncorporatedBlockID, ir.Result.ID()})
}

// CheckSum implements flow.Entity.CheckSum for IncorporatedResult to make it
// capable of being stored directly in mempools and storage.
func (ir *IncorporatedResult) Checksum() Identifier {
	return MakeID(ir)
}

// GetSignature returns a signature by chunk index and signer ID.
func (ir *IncorporatedResult) GetSignature(chunkIndex uint64, signerID Identifier) (*crypto.Signature, bool) {
	ir.aggregatedSignaturesLock.Lock()
	defer ir.aggregatedSignaturesLock.Unlock()

	as, ok := ir.aggregatedSignatures[chunkIndex]
	if !ok {
		return nil, false
	}
	return as.BySigner(signerID)
}

// AddSignature adds a signature to the collection of AggregatedSignatures
func (ir *IncorporatedResult) AddSignature(chunkIndex uint64, signerID Identifier, signature crypto.Signature) {
	ir.aggregatedSignaturesLock.Lock()
	defer ir.aggregatedSignaturesLock.Unlock()

	as, ok := ir.aggregatedSignatures[chunkIndex]
	if !ok {
		as = &AggregatedSignature{}
	}

	as.Add(signerID, signature)

	ir.aggregatedSignatures[chunkIndex] = as
}

// GetAggregatedSignatures returns all the aggregated signatures orderd by chunk
// index
func (ir *IncorporatedResult) GetAggregatedSignatures() []AggregatedSignature {
	ir.aggregatedSignaturesLock.Lock()
	defer ir.aggregatedSignaturesLock.Unlock()

	result := make([]AggregatedSignature, 0, len(ir.Result.Chunks))

	for _, chunk := range ir.Result.Chunks {
		as, ok := ir.aggregatedSignatures[chunk.Index]
		if ok {
			result = append(result, *as)
		}
	}

	return result
}
