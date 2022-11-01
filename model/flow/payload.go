package flow

import (
	"encoding/json"

	"github.com/onflow/flow-go/model"
)

// Payload is the actual content of each block.
type Payload struct {
	// Guarantees are ordered in execution order.
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
	return json.Marshal(struct{ payloadAlias }{
		payloadAlias: payloadAlias(p),
	})
}

func (p *Payload) StructureValid() error {
	if p == nil {
		return model.NewStructureInvalidError("nil payload")
	}
	for _, guarantee := range p.Guarantees {
		if guarantee == nil {
			return model.NewStructureInvalidError("contains nil guarantee")
		}
	}
	for _, seal := range p.Seals {
		if seal == nil {
			return model.NewStructureInvalidError("contains nil seal")
		}
	}
	for _, receipt := range p.Receipts {
		if receipt == nil {
			return model.NewStructureInvalidError("contains nil receipt")
		}
	}
	for _, result := range p.Results {
		if result == nil {
			return model.NewStructureInvalidError("contains nil result")
		}
	}
	return nil
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
