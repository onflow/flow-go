package flow

// Payload is the actual content of each block.
type Payload struct {
	Guarantees []*CollectionGuarantee
	Seals      []*Seal
	Receipts   []*ExecutionReceipt
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
	}
	return idx
}
