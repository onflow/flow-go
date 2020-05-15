package flow

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/encoding/rlp"
)

// Attestation confirms correctness of a chunk of an exec result
type Attestation struct {
	BlockID           Identifier // ID of the block included the collection
	ExecutionResultID Identifier // ID of the execution result
	ChunkIndex        uint64     // index of the approved chunk
}

// ID generates a unique identifier using attestation
func (a Attestation) ID() Identifier {
	return MakeID(a)
}

// Encode serializes the Attestation.
func (a Attestation) Encode() []byte {
	encoder := rlp.NewEncoder()
	return encoder.MustEncode(a)
}

// ResultApprovalBody holds body part of a result approval
type ResultApprovalBody struct {
	Attestation
	ApproverID           Identifier       // node id generating this result approval
	AttestationSignature crypto.Signature // signature over attestation, this has been separated for BLS aggregation
	Spock                Spock            // proof of re-computation, one per each chunk
}

// Encode serializes the ResultApprovalBody.
func (rab ResultApprovalBody) Encode() []byte {
	encoder := rlp.NewEncoder()
	return encoder.MustEncode(rab)
}

// ID generates a unique identifier using ResultApprovalBody
func (rab ResultApprovalBody) ID() Identifier {
	return MakeID(rab)
}

// ResultApproval includes an approval for a chunk, verified by a verification node
type ResultApproval struct {
	Body              ResultApprovalBody
	VerifierSignature crypto.Signature // signature over all above fields
}

// ID generates a unique identifier using result approval body
func (ra ResultApproval) ID() Identifier {
	return MakeID(ra.Body)
}

// Checksum generates checksum using the result approval full content
func (ra ResultApproval) Checksum() Identifier {
	return MakeID(ra)
}
