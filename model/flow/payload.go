package flow

import (
	"encoding/json"
)

// Payload is the actual content of each block.
type Payload struct {
	// Guarantees are ordered in execution order. May be empty, in which case
	// only the system chunk is executed for this block.
	Guarantees []*CollectionGuarantee
	// Seals holds block seals for ancestor blocks.
	// The oldest seal must connect to the latest seal in the fork extended by this block.
	// Seals must be internally connected, containing no seals with duplicate block IDs or heights.
	// Seals may be empty. It presents a set, i.e. there is no protocol-defined ordering.
	Seals    []*Seal
	Receipts ExecutionReceiptMetaList
	Results  ExecutionResultList
	// ProtocolStateID is the root hash of protocol state. Per convention, this is the resulting
	// state after applying all identity-changing operations potentially contained in the block.
	// The block payload itself is validated wrt to the protocol state committed to by its parent.
	// Thereby, we are only  accepting protocol states that have been certified by a valid QC.
	ProtocolStateID Identifier
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

// Hash returns the root hash of the payload.
func (p Payload) Hash() Identifier {
	collHash := MerkleRoot(GetIDs(p.Guarantees)...)
	sealHash := MerkleRoot(GetIDs(p.Seals)...)
	recHash := MerkleRoot(GetIDs(p.Receipts)...)
	resHash := MerkleRoot(GetIDs(p.Results)...)
	return ConcatSum(collHash, sealHash, recHash, resHash, p.ProtocolStateID)
}

// Index returns the index for the payload.
func (p Payload) Index() *Index {
	idx := &Index{
		CollectionIDs:   GetIDs(p.Guarantees),
		SealIDs:         GetIDs(p.Seals),
		ReceiptIDs:      GetIDs(p.Receipts),
		ResultIDs:       GetIDs(p.Results),
		ProtocolStateID: p.ProtocolStateID,
	}
	return idx
}
