package flow

// Payload is the actual content of each block.
type Payload struct {
	Guarantees []*CollectionGuarantee
	Seals      []*Seal
}

// Hash returns the root hash of the payload.
func (p Payload) Hash() Identifier {
	collHash := MerkleRoot(GetIDs(p.Guarantees)...)
	sealHash := MerkleRoot(GetIDs(p.Seals)...)
	return ConcatSum(collHash, sealHash)
}

// Index returns the index for the payload.
func (p Payload) Index() *Index {
	idx := &Index{
		CollectionIDs: GetIDs(p.Guarantees),
		SealIDs:       GetIDs(p.Seals),
	}
	return idx
}
