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

// MarshalJSON defines the JSON marshalling for block payloads. Enforce a
// consistent representation for empty slices.
func (p Payload) MarshalJSON() ([]byte, error) {
	if len(p.Guarantees) == 0 {
		p.Guarantees = nil
	}
	if len(p.Receipts) == 0 {
		p.Receipts = nil
	}
	if len(p.Seals) == 0 {
		p.Seals = nil
	}
	if len(p.Results) == 0 {
		p.Results = nil
	}

	type payloadAlias Payload
	return json.Marshal(struct {payloadAlias} {
		payloadAlias: payloadAlias(p),
	})
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
