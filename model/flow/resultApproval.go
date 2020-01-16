package flow

import (
	"github.com/dapperlabs/flow-go/crypto"
)

type ResultApprovalBody struct {
	// CorrectnessAttestation
	ExecutionResultID    Identifier       //hash of approved execution result
	AttestationSignature crypto.Signature // signature over ExecutionResultHash

	// Verification Proof
	ChunkIndexList []uint32 // list of chunk indices assigned tot he verifier
	Proof          []byte   // proof of correctness of the chunk assignment
	Spocks         []Spock  // proof of re-computation, one per each chunk
}

type ResultApproval struct {
	ResultApprovalBody ResultApprovalBody
	VerifierSignature  crypto.Signature // signature over all above fields
}

func (ra ResultApproval) Body() interface{} {
	return ra.ResultApprovalBody
}

func (ra ResultApproval) ID() Identifier {
	return MakeID(ra.ResultApprovalBody)
}

func (ra ResultApproval) Checksum() Identifier {
	return MakeID(ra)
}
