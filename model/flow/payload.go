package flow

// Payload is the actual content of each block.
type Payload struct {
	Identities IdentityList
	Guarantees []*CollectionGuarantee
	Seals      []*Seal
}

// Hash returns the root hash of the payload.
func (p Payload) Hash() Identifier {
	idHash := MerkleRoot(GetIDs(p.Identities)...)
	collHash := MerkleRoot(GetIDs(p.Guarantees)...)
	sealHash := MerkleRoot(GetIDs(p.Seals)...)
	return ConcatSum(idHash, collHash, sealHash)
}

// Index returns the index for the payload.
func (p Payload) Index() *Index {
	idx := &Index{
		NodeIDs:       GetIDs(p.Identities),
		CollectionIDs: GetIDs(p.Guarantees),
		SealIDs:       GetIDs(p.Seals),
	}
	return idx
}
