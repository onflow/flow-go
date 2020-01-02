package flow

import (
	"bytes"

	"encoding/gob"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/encoding"
)

type ResultApprovalBody struct {
	// CorrectnessAttestation
	ExecutionResultHash Fingerprint       //hash of approved execution result
	AttestationSignature crypto.Signature // signature over ExecutionResultHash

	// Verification Proof
	ChunkIndexList  []uint32 // list of chunk indices assigned tot he verifier
	Proof           []byte   // proof of correctness of the chunk assignment
	Spocks          []Spock  // proof of re-computation, one per each chunk
}

type ResultApproval struct {
	Body ResultApprovalBody
	VerifierSignature crypto.Signature // signature over all above fields
}

// Encode implements the crypto.Encoder interface.
func (ra *ResultApproval) Encode() []byte {
	var b bytes.Buffer
	e := gob.NewEncoder(&b)
	if err := e.Encode(ra); err != nil {
		panic(err)
	}
	return b.Bytes()
}

// Hash returns the canonical hash of this execution receipt.
func (ra *ResultApproval) Fingerprint() Fingerprint {
	return encoding.DefaultEncoder.MustEncode(ra.Body)
}
