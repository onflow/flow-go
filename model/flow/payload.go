package flow

import (
	"encoding/json"
	"fmt"
)

// Payload is the actual content of each block.
//
//structwrite:immutable - mutations allowed only within the constructor
type Payload struct {
	// Guarantees are ordered in execution order. May be empty, in which case
	// only the system chunk is executed for this block.
	Guarantees []*CollectionGuarantee
	// Seals holds block seals for ancestor blocks.
	// The oldest seal must connect to the latest seal in the fork extended by this block.
	// Seals must be internally connected, containing no seals with duplicate block IDs or heights.
	// Seals may be empty. It presents a set, i.e. there is no protocol-defined ordering.
	Seals    []*Seal
	Receipts ExecutionReceiptStubList
	Results  ExecutionResultList
	// ProtocolStateID is the root hash of protocol state. Per convention, this is the resulting
	// state after applying all identity-changing operations potentially contained in the block.
	// The block payload itself is validated wrt to the protocol state committed to by its parent.
	// Thereby, we are only accepting protocol states that have been certified by a valid QC.
	ProtocolStateID Identifier
}

// UntrustedPayload is an untrusted input-only representation of the main consensus Payload,
// used for construction.
//
// This type exists to ensure that constructor functions are invoked explicitly
// with named fields, which improves clarity and reduces the risk of incorrect field
// ordering during construction.
//
// An instance of UntrustedPayload should be validated and converted into
// a trusted main consensus Payload using NewPayload constructor.
type UntrustedPayload Payload

// NewPayload creates a new payload.
// Construction of Payload is allowed only within the constructor.
//
// All errors indicate a valid Payload cannot be constructed from the input.
func NewPayload(untrusted UntrustedPayload) (*Payload, error) {
	if untrusted.ProtocolStateID == ZeroID {
		return nil, fmt.Errorf("ProtocolStateID must not be zero")
	}

	return &Payload{
		Guarantees:      untrusted.Guarantees,
		Seals:           untrusted.Seals,
		Receipts:        untrusted.Receipts,
		Results:         untrusted.Results,
		ProtocolStateID: untrusted.ProtocolStateID,
	}, nil
}

// NewEmptyPayload returns an empty block payload.
func NewEmptyPayload() *Payload {
	return &Payload{}
}

// MarshalJSON defines the JSON marshalling for block payloads. Enforce a
// consistent representation for empty slices.
func (p Payload) MarshalJSON() ([]byte, error) {
	if len(p.Guarantees) == 0 {
		// It is safe to mutate here because this is part of custom JSON marshaling logic.
		//nolint:structwrite
		p.Guarantees = nil
	}
	if len(p.Receipts) == 0 {
		// It is safe to mutate here because this is part of custom JSON marshaling logic.
		//nolint:structwrite
		p.Receipts = nil
	}
	if len(p.Seals) == 0 {
		// It is safe to mutate here because this is part of custom JSON marshaling logic.
		//nolint:structwrite
		p.Seals = nil
	}
	if len(p.Results) == 0 {
		// It is safe to mutate here because this is part of custom JSON marshaling logic.
		//nolint:structwrite
		p.Results = nil
	}

	type payloadAlias Payload
	return json.Marshal(struct{ payloadAlias }{
		payloadAlias: payloadAlias(p),
	})
}

// Hash returns the root hash of the payload.
func (p Payload) Hash() Identifier {
	guaranteesHash := MerkleRoot(GetIDs(p.Guarantees)...)
	sealHash := MerkleRoot(GetIDs(p.Seals)...)
	recHash := MerkleRoot(GetIDs(p.Receipts)...)
	resHash := MerkleRoot(GetIDs(p.Results)...)
	return ConcatSum(guaranteesHash, sealHash, recHash, resHash, p.ProtocolStateID)
}

// Index returns the index for the payload.
func (p Payload) Index() *Index {
	idx := &Index{
		GuaranteeIDs:    GetIDs(p.Guarantees),
		SealIDs:         GetIDs(p.Seals),
		ReceiptIDs:      GetIDs(p.Receipts),
		ResultIDs:       GetIDs(p.Results),
		ProtocolStateID: p.ProtocolStateID,
	}
	return idx
}
