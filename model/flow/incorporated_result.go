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
	chunkApprovals     map[uint64]*SignatureCollector
	chunkApprovalsLock sync.Mutex
}

func NewIncorporatedResult(incorporatedBlockID Identifier, result *ExecutionResult) *IncorporatedResult {
	return &IncorporatedResult{
		IncorporatedBlockID: incorporatedBlockID,
		Result:              result,
		chunkApprovals:      make(map[uint64]*SignatureCollector),
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
	as := s.ToAggregatedSignature()
	return &as, true
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
		c := NewSignatureCollector()
		as = &c
		ir.chunkApprovals[chunkIndex] = as
	}

	as.Add(signerID, signature)
}

// NumberSignatures returns the number of stored (distinct) signatures for the given chunk
func (ir *IncorporatedResult) NumberSignatures(chunkIndex uint64) uint {
	ir.chunkApprovalsLock.Lock()
	defer ir.chunkApprovalsLock.Unlock()

	as, ok := ir.chunkApprovals[chunkIndex]
	if !ok {
		return 0
	}
	return as.NumberSignatures()
}

// GetAggregatedSignatures returns all the aggregated signatures orderd by chunk
// index
func (ir *IncorporatedResult) GetAggregatedSignatures() []AggregatedSignature {
	ir.chunkApprovalsLock.Lock()
	defer ir.chunkApprovalsLock.Unlock()

	result := make([]AggregatedSignature, 0, len(ir.Result.Chunks))

	for _, chunk := range ir.Result.Chunks {
		ca, ok := ir.chunkApprovals[chunk.Index]
		if ok {
			result = append(result, ca.ToAggregatedSignature())
		} else {
			result = append(result, AggregatedSignature{})
		}
	}

	return result
}

/* ************************************************************************ */

// SignatureCollector contains a set of of signatures from verifiers attesting
// to the validity of an execution result chunk.
// NOT concurrency safe.
// TODO: this will be replaced with stateful BLS aggregation
type SignatureCollector struct {
	// List of signatures
	verifierSignatures []crypto.Signature
	// List of signer identifiers
	signerIDs []Identifier

	// set of all signerIDs for de-duplicating signatures; the mapped value
	// is the storage index in the verifierSignatures and signerIDs
	signerIDSet map[Identifier]int
}

// NewSignatureCollector instantiates a new SignatureCollector
func NewSignatureCollector() SignatureCollector {
	return SignatureCollector{
		verifierSignatures: nil,
		signerIDs:          nil,
		signerIDSet:        make(map[Identifier]int),
	}
}

// ToAggregatedSignature generates an aggregated signature from all signatures
// in the SignatureCollector
func (c *SignatureCollector) ToAggregatedSignature() AggregatedSignature {
	signatures := make([]crypto.Signature, len(c.verifierSignatures))
	copy(signatures, c.verifierSignatures)

	signers := make([]Identifier, len(c.signerIDs))
	copy(signers, c.signerIDs)

	return AggregatedSignature{
		VerifierSignatures: signatures,
		SignerIDs:          signers,
	}
}

// BySigner returns a signer's signature if it exists
func (c *SignatureCollector) BySigner(signerID Identifier) (*crypto.Signature, bool) {
	idx, found := c.signerIDSet[signerID]
	if !found {
		return nil, false
	}
	return &c.verifierSignatures[idx], true
}

// HasSigned checks if signer has already provided a signature
func (c *SignatureCollector) HasSigned(signerID Identifier) bool {
	_, found := c.signerIDSet[signerID]
	return found
}

// Add appends a signature. Only the _first_ signature is retained for each signerID.
func (c *SignatureCollector) Add(signerID Identifier, signature crypto.Signature) {
	if _, found := c.signerIDSet[signerID]; found {
		return
	}
	c.signerIDSet[signerID] = len(c.signerIDs)
	c.signerIDs = append(c.signerIDs, signerID)
	c.verifierSignatures = append(c.verifierSignatures, signature)
}

// NumberSignatures returns the number of stored (distinct) signatures
func (c *SignatureCollector) NumberSignatures() uint {
	return uint(len(c.signerIDs))
}

/*******************************************************************************
GROUPING allows to split a list incorporated results by some property
*******************************************************************************/

// IncorporatedResultList is a slice of IncorporatedResults with the additional
// functionality to group them by various properties
type IncorporatedResultList []*IncorporatedResult

// IncorporatedResultGroupedList is a partition of an IncorporatedResultList
type IncorporatedResultGroupedList map[Identifier]IncorporatedResultList

// IncorporatedResultGroupingFunction is a function that assigns an identifier to each IncorporatedResult
type IncorporatedResultGroupingFunction func(*IncorporatedResult) Identifier

// GroupBy partitions the IncorporatedResultList. All IncorporatedResults that are
// mapped by the grouping function to the same identifier are placed in the same group.
// Within each group, the order and multiplicity of the IncorporatedResults is preserved.
func (l IncorporatedResultList) GroupBy(grouper IncorporatedResultGroupingFunction) IncorporatedResultGroupedList {
	groups := make(map[Identifier]IncorporatedResultList)
	for _, ir := range l {
		groupID := grouper(ir)
		groups[groupID] = append(groups[groupID], ir)
	}
	return groups
}

// GroupByIncorporatedBlockID partitions the IncorporatedResultList by the ID of the block that
// incorporates the result. Within each group, the order and multiplicity of the
// IncorporatedResults is preserved.
func (l IncorporatedResultList) GroupByIncorporatedBlockID() IncorporatedResultGroupedList {
	grouper := func(ir *IncorporatedResult) Identifier { return ir.IncorporatedBlockID }
	return l.GroupBy(grouper)
}

// GroupByResultID partitions the IncorporatedResultList by the Results' IDs.
// Within each group, the order and multiplicity of the IncorporatedResults is preserved.
func (l IncorporatedResultList) GroupByResultID() IncorporatedResultGroupedList {
	grouper := func(ir *IncorporatedResult) Identifier { return ir.Result.ID() }
	return l.GroupBy(grouper)
}

// GroupByExecutedBlockID partitions the IncorporatedResultList by the IDs of the executed blocks.
// Within each group, the order and multiplicity of the IncorporatedResults is preserved.
func (l IncorporatedResultList) GroupByExecutedBlockID() IncorporatedResultGroupedList {
	grouper := func(ir *IncorporatedResult) Identifier { return ir.Result.BlockID }
	return l.GroupBy(grouper)
}

// Size returns the number of IncorporatedResults in the list
func (l IncorporatedResultList) Size() int {
	return len(l)
}

// GetGroup returns the IncorporatedResults that were mapped to the same identifier by the
// grouping function. Returns an empty (nil) IncorporatedResultList if groupID does not exist.
func (g IncorporatedResultGroupedList) GetGroup(groupID Identifier) IncorporatedResultList {
	return g[groupID]
}

// NumberGroups returns the number of groups
func (g IncorporatedResultGroupedList) NumberGroups() int {
	return len(g)
}
