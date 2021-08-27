package signature

import (
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
)

// RandomBeaconReconstructor implements hotstuff.RandomBeaconReconstructor.
// The implementation wraps the thresholdSigner and translates the signer identity into signer index.
type RandomBeaconReconstructor struct {
	dkg                hotstuff.DKG                // to lookup signer index by signer ID
	randomBeaconSigner hotstuff.RandomBeaconSigner // a stateful object for this block. It's used for both storing all sig shares and producing the node's own share by signing the block
}

var _ hotstuff.RandomBeaconReconstructor = &RandomBeaconReconstructor{}

func NewRandomBeaconReconstructur(dkg hotstuff.DKG, randomBeaconSigner hotstuff.RandomBeaconSigner) *RandomBeaconReconstructor {
	return &RandomBeaconReconstructor{
		dkg:                dkg,
		randomBeaconSigner: randomBeaconSigner,
	}
}

// Verify returns true if and only if the signature is valid.
// It expects that correct type of signature is passed. Only SigTypeRandomBeacon is supported
func (r *RandomBeaconReconstructor) Verify(signerID flow.Identifier, sig crypto.Signature) (bool, error) {
	panic("to be implemented")
}

// TrustedAdd adds the signature share to the reconstructors internal
// state. Validity of signature is not checked. It is up to the
// implementation, whether it still adds a signature or not, when the
// minimal number of required sig shares has already been reached,
// because the reconstructed group signature is the same.
// Returns: true if and only if enough signature shares were collected
func (r *RandomBeaconReconstructor) TrustedAdd(signerID flow.Identifier, sig crypto.Signature) (bool, error) {
	panic("to be implemented")
}

// HasSufficientShares returns true if and only if reconstructor
// has collected a sufficient number of signature shares.
func (r *RandomBeaconReconstructor) HasSufficientShares() bool {
	panic("to be implemented")
}

// Reconstruct reconstructs the group signature from the provided
// signature shares. Errors if the the number of shares is insufficient
// or some of the added signatures shares were invalid.
func (r *RandomBeaconReconstructor) Reconstruct() (crypto.Signature, error) {
	panic("to be implemented")
}
