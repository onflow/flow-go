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

	// chunkApprovals is a placeholder for attestation signatures
	// collected for each chunk. It gets populated by the consensus matching
	// engine when approvals are matched to execution results.
	// This field is not exported (name doesn't start with a capital letter), so
	// it is not used in calculating the ID and Checksum of the Incorporated
	// Result (RLP encoding ignores private fields).
	chunkApprovals     map[uint64]*approverSignatures
	chunkApprovalsLock sync.Mutex
}

func NewIncorporatedResult(incorporatedBlockID Identifier, result *ExecutionResult) *IncorporatedResult {
	return &IncorporatedResult{
		IncorporatedBlockID: incorporatedBlockID,
		Result:              result,
		chunkApprovals:      make(map[uint64]*approverSignatures),
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

// GetChunkSignatures returns the AggregatedSignature for a specific chunk
func (ir *IncorporatedResult) GetChunkSignatures(chunkIndex uint64) (*AggregatedSignature, bool) {
	ir.chunkApprovalsLock.Lock()
	defer ir.chunkApprovalsLock.Unlock()
	s, ok := ir.chunkApprovals[chunkIndex]
	if !ok {
		return nil, false
	}
	return s.ToAggregatedSignature(), true
}

// GetSignature returns a signature by chunk index and signer ID
func (ir *IncorporatedResult) GetSignature(chunkIndex uint64, signerID Identifier) (*crypto.Signature, bool) {
	ir.chunkApprovalsLock.Lock()
	defer ir.chunkApprovalsLock.Unlock()

	as, ok := ir.chunkApprovals[chunkIndex]
	if !ok {
		return nil, false
	}
	return as.BySigner(signerID)
}

// AddSignature adds a signature to the collection of AggregatedSignatures
func (ir *IncorporatedResult) AddSignature(chunkIndex uint64, signerID Identifier, signature crypto.Signature) {
	ir.chunkApprovalsLock.Lock()
	defer ir.chunkApprovalsLock.Unlock()

	as, ok := ir.chunkApprovals[chunkIndex]
	if !ok {
		as = NewApproverSignatures()
		ir.chunkApprovals[chunkIndex] = as
	}

	as.Add(signerID, signature)
}

// GetAggregatedSignatures returns all the aggregated signatures orderd by chunk
// index
func (ir *IncorporatedResult) GetAggregatedSignatures() []AggregatedSignature {
	ir.chunkApprovalsLock.Lock()
	defer ir.chunkApprovalsLock.Unlock()

	result := make([]AggregatedSignature, 0, len(ir.Result.Chunks))

	for _, chunk := range ir.Result.Chunks {
		as, ok := ir.chunkApprovals[chunk.Index]
		if ok {
			result = append(result, *as.ToAggregatedSignature())
		} else {
			result = append(result, AggregatedSignature{})
		}
	}

	return result
}

/* ************************************************************************ */

// approverSignatures contains a set of of signatures from verifiers attesting
// to the validity of an execution result chunk.
// TODO: this will be replaced with BLS aggregation
type approverSignatures struct {
	// List of signatures
	verifierSignatures []crypto.Signature
	// List of signer identifiers
	signerIDs []Identifier

	signerIDSet map[Identifier]struct{}
}

func NewApproverSignatures() *approverSignatures {
	return &approverSignatures{
		verifierSignatures: nil,
		signerIDs:          nil,
		signerIDSet:        make(map[Identifier]struct{}),
	}
}

func (as *approverSignatures) ToAggregatedSignature() *AggregatedSignature {
	signatures := make([]crypto.Signature, len(as.verifierSignatures))
	copy(signatures, as.verifierSignatures)

	signers := make([]Identifier, len(as.signerIDs))
	copy(signers, as.signerIDs)

	return &AggregatedSignature{
		VerifierSignatures: signatures,
		SignerIDs:          signers,
	}
}

// BySigner returns a signer's signature if it exists
func (as *approverSignatures) BySigner(signerID Identifier) (*crypto.Signature, bool) {
	for index, id := range as.signerIDs {
		if id == signerID {
			return &as.verifierSignatures[index], true
		}
	}
	return nil, false
}

// Add appends a signature.
// ATTENTION: it does not check for duplicates
func (as *approverSignatures) Add(signerID Identifier, signature crypto.Signature) {
	if _, found := as.signerIDSet[signerID]; found {
		return
	}
	as.signerIDSet[signerID] = struct{}{}
	as.signerIDs = append(as.signerIDs, signerID)
	as.verifierSignatures = append(as.verifierSignatures, signature)
}
