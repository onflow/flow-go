package flow

// Payload is the actual content of each block.
type Payload struct {
	Guarantees []*CollectionGuarantee
	Seals      []*Seal
	Receipts   []*ExecutionReceiptMeta
	Results    []*ExecutionResult
}

// EmptyPayload returns an empty block payload.
func EmptyPayload() Payload {
	return Payload{}
}

// Hash returns the root hash of the payload.
func (p Payload) Hash() Identifier {
	collHash := MerkleRoot(GetIDs(p.Guarantees)...)
	sealHash := MerkleRoot(GetIDs(p.Seals)...)
	recHash := MerkleRoot(GetIDs(p.Receipts)...)
	return ConcatSum(collHash, sealHash, recHash)
}

// Index returns the index for the payload.
func (p Payload) Index() *Index {
	idx := &Index{
		CollectionIDs: GetIDs(p.Guarantees),
		SealIDs:       GetIDs(p.Seals),
		ReceiptIDs:    GetIDs(p.Receipts),
		ResultIDs:     GetIDs(p.Results),
	}
	return idx
}

// ResultsById generates a lookup map for accesing execution results by ID.
func (p Payload) ResultsById() map[Identifier]*ExecutionResult {
	resultsByID := make(map[Identifier]*ExecutionResult)
	for _, result := range p.Results {
		resultsByID[result.ID()] = result
	}
	return resultsByID
}
