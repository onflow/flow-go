package flow

import (
	"encoding/json"
)

// Payload is the actual content of each block.
type Payload struct {
	Guarantees []*CollectionGuarantee
	Seals      []*Seal
	Receipts   ExecutionReceiptMetaList
	Results    ExecutionResultList
}

// EmptyPayload returns an empty block payload.
func EmptyPayload() Payload {
	return Payload{}
}

// JSONMarshal defines the JSON marshalling for block payloads. Enforce a
// consistent representation for empty slices.
func (p *Payload) MarshalJSON() ([]byte, error) {
	dup := *p // copy p

	if len(dup.Guarantees) == 0 {
		dup.Guarantees = nil
	}
	if len(dup.Receipts) == 0 {
		dup.Receipts = nil
	}
	if len(dup.Seals) == 0 {
		dup.Seals = nil
	}
	if len(dup.Results) == 0 {
		dup.Results = nil
	}

	return json.Marshal(dup)
}

// Hash returns the root hash of the payload.
func (p Payload) Hash() Identifier {
	collHash := MerkleRoot(GetIDs(p.Guarantees)...)
	sealHash := MerkleRoot(GetIDs(p.Seals)...)
	recHash := MerkleRoot(GetIDs(p.Receipts)...)
	resHash := MerkleRoot(GetIDs(p.Results)...)
	return ConcatSum(collHash, sealHash, recHash, resHash)
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
