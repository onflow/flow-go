package votecollector

import "github.com/onflow/flow-go/crypto"

type randomBeaconReconstructor struct {
}

// Verify returns true if and only if the signature is valid.
func (rbr *randomBeaconReconstructor) Verify(signerID interface{}, sig crypto.Signature) (bool, error) {
	panic("implement me")
}

// TrustedAdd adds the signature share to the reconstructors internal
// state. Validity of signature is not checked. It is up to the
// implementation, wheter it still adds a signature or not, when the
// minimal number of required sig shares has already been reached,
// because the reconstructed group signature is the same.
// Returns: true if and only if enough signature shares were collected
func (rbr *randomBeaconReconstructor) TrustedAdd(signerIndex uint, sigShare crypto.Signature) (bool, error) {
	panic("implement me")
}

// HasSufficientShares returns true if and only if reconstructor
// has collected a sufficient number of signature shares.
func (rbr *randomBeaconReconstructor) HasSufficientShares() bool {
	panic("implement me")
}

// Reconstruct reconstructs the group signature from the provided
// signature shares. Errors if the the number of shares is insufficient
// or some of the added signatures shares were invalid.
func (rbr *randomBeaconReconstructor) Reconstruct() (crypto.Signature, error) {
	panic("implement me")
}
