package hotstuff

import "github.com/onflow/flow-go/crypto"

type ThresholdSigner interface {
	// Verify verifies whether the signature is from the signer specified by the given signer index
	// it allows concurrent verification of the given signature
	// return nil if signature is valid
	// return crypto.InvalidInputsError if the signature is invalid
	// return other error if there is exception
	Verify(signerIndex int, share crypto.Signature) error

	// TrustedAdd adds a verified share to the internal signature shares store
	// The operation is sequential.
	// It assumes the signature share has been verified and is valid.
	// return (true, nil) if the signature has been added
	// return (false, nil) if the signature is a duplication
	// return (false, error) if there is any exception
	// TODO: should we let it also return EnoughShares?, because otherwise
	// calling EnoughShares might return true as if the vote was the last one
	// to reach the threshold, but actually there was another vote got added in
	// a different thread.
	TrustedAdd(signerIndex int, share crypto.Signature) (bool, error)

	// VerifyAndAdd combines Verify and TrustedAdd into one call.
	// If called concurrently, it is able to concurrently verifies the signature
	// but sequentially adding the signature shares to it's internal store.
	VerifyAndAdd(signerIndex int, share crypto.Signature) (bool, bool, error)

	// EnoughShares returns whether it has accumulated enough shares to reconstruct
	// a group signature
	EnoughShares() bool

	// SignShare produces a signature share with its own private key.
	SignShare() (crypto.Signature, error)

	// Reconstruct reconstructs the group signature.
	// It assumes the threshold enough shares have been collected.
	Reconstruct() (crypto.Signature, error)
}

// This provides a mock implementation of ThresholdSigner in crypto/thresholdsign.go
// TODO: remove when crypto/thresholdsign supports concurrent calls.
type ThresholdSignerImpl struct {
	// dependencies
	msg             []byte
	threshold       int
	groupPublicKey  crypto.PublicKey
	myIndex         int
	myPrivateKey    crypto.PrivateKey
	publicKeyShares []crypto.PublicKey

	// state
	haveEnoughShares   bool
	signers            []int
	thresholdSignature crypto.Signature
}
