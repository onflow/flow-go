package hotstuff

import (
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
)

// RandomBeaconReconstructor collects verified signature shares, and reconstructs the
// group signature with enough shares.
type RandomBeaconReconstructor interface {
	// TrustedAdd adds the signature share to the reconstructors internal
	// state. Validity of signature is not checked. It is up to the
	// implementation, whether it still adds a signature or not, when the
	// minimal number of required sig shares has already been reached,
	// because the reconstructed group signature is the same.
	// It returns:
	//  - (true, nil) if and only if enough signature shares were collected
	//  - (false, nil) if not enough shares were collected
	//  - (false, error) if there is exception adding the sig share)
	TrustedAdd(signerID flow.Identifier, sig crypto.Signature) (hasSufficientShares bool, err error)

	// HasSufficientShares returns true if and only if reconstructor
	// has collected a sufficient number of signature shares.
	HasSufficientShares() bool

	// Reconstruct reconstructs the group signature from the provided
	// signature shares. Errors if the the number of shares is insufficient
	// or some of the added signatures shares were invalid.
	Reconstruct() (crypto.Signature, error)
}

// SigType is the aggregable signature type.
type SigType int

// There are two signatures are aggregable, one is the normal staking signature,
// the other is the threshold sig used as staking sigature.
const (
	SigTypeStaking SigType = iota
	SigTypeRandomBeacon
	SigTypeInvalid
)

// SignatureAggregator aggregates the verified signatures.
// It can either be used to aggregate staking signatures or aggregate random beacon signatures,
// but not a mix of staking signature and random beacon signature.
// Implementation of SignatureAggregator must be concurrent safe
type SignatureAggregator interface {
	// TrustedAdd adds an already verified signature, and look up the weight for the given signer,
	// and add it to the total weight, and returns the total weight that have been collected.
	// return (1000, nil) means the signature has been added, and 1000 weight has been collected in total.
	//   (1000 is just an example)
	// return (1000, nil) means the signature is a duplication and 1000 weight has been collected in total.
	TrustedAdd(signerID flow.Identifier, sig crypto.Signature) (totalWeight uint64, exception error)

	// TotalWeight returns the total weight presented by the collected sig shares.
	TotalWeight() uint64

	// Aggregate assumes enough shares have been collected, it aggregates the signatures
	// and return the aggregated signature.
	// if called concurrently, only one thread will be running the aggregation.
	// TODO: consider return (signerIDs and crypto.Signature)
	Aggregate() ([]byte, error)
}

// AggregatedSignatureData is an intermediate struct for Packer to pack the
// aggregated signature data into raw bytes or unpack from raw bytes.
type AggregatedSignatureData struct {
	StakingSigners               []flow.Identifier
	RandomBeaconSigners          []flow.Identifier
	AggregatedStakingSig         crypto.Signature
	AggregatedRandomBeaconSig    crypto.Signature
	ReconstructedRandomBeaconSig crypto.Signature
}

// Packer packs aggregated signature data into raw bytes to be used in block header.
type Packer interface {
	Pack(sig *AggregatedSignatureData) ([]flow.Identifier, []byte, error)

	Unpack(signerIDs []flow.Identifier, sigData []byte) (*AggregatedSignatureData, error)
}
