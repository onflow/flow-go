package hotstuff

import (
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
)

// RandomBeaconReconstructor collects verified signature shares, and reconstructs the
// group signature with enough shares.
type RandomBeaconReconstructor interface {
	// Verify verifies the signature under the stored public key corresponding to the signerID, and the stored message agreed about upfront.
	Verify(signerID flow.Identifier, sig crypto.Signature) (bool, error)

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

	// Reconstruct reconstructs the group signature.
	// The reconstructed signature is verified against the overall group public key and the message agreed upon.
	// This is a sanity check that is necessary since "TrustedAdd" allows adding non-verified signatures.
	// Reconstruct returns an error if the reconstructed signature fails the sanity verification, or if not enough shares have been collected.
	Reconstruct() (crypto.Signature, error)
}

// SigType is the aggregable signature type.
type SigType uint8

// SigType specifies the role of the signature in the protocol. SigTypeRandomBeacon type is for random beacon signatures. SigTypeStaking is for Hotstuff sigantures. Both types are aggregatable cryptographic signatures.
const (
	SigTypeStaking SigType = iota
	SigTypeRandomBeacon
)

// Valid returns true if the signature is either SigTypeStaking or SigTypeRandomBeacon
// else return false
func (t SigType) Valid() bool {
	return t == SigTypeStaking || t == SigTypeRandomBeacon
}

// WeightedSignatureAggregator aggregates signatures of the same signature scheme from different signers.
// The module is aware of weights assigned to each signer.
// It can either be used to aggregate staking signatures or aggregate random beacon signatures,
// but not a mix of both.
// Implementation of SignatureAggregator must be concurrent safe.
type WeightedSignatureAggregator interface {
	// Verify verifies the signature under the stored public key corresponding to the signerID, and the stored message.
	Verify(signerID flow.Identifier, sig crypto.Signature) (bool, error)

	// TrustedAdd adds an already verified signature, with weight for the given signer,
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
	// Aggregate attempts to aggregate the internal signatures and returns the resulting signature data.
	// It errors if not enough weights have been collected.
	// The function performs a final verification and errors if the aggregated signature is not valid. This is
	// required for the function safety since "TrustedAdd" allows adding invalid signatures.
	// If called concurrently, only one thread will be running the aggregation.
	Aggregate() ([]flow.Identifier, crypto.Signature, error)
}

// BlockSignatureData is an intermediate struct for Packer to pack the
// aggregated signature data into raw bytes or unpack from raw bytes.
type BlockSignatureData struct {
	StakingSigners               []flow.Identifier
	RandomBeaconSigners          []flow.Identifier
	AggregatedStakingSig         []byte
	AggregatedRandomBeaconSig    []byte
	ReconstructedRandomBeaconSig crypto.Signature
}

// Packer packs aggregated signature data into raw bytes to be used in block header.
type Packer interface {
	// blockID is the block that the aggregated sig is for
	// sig is the aggregated signature data
	Pack(blockID flow.Identifier, sig *BlockSignatureData) ([]flow.Identifier, []byte, error)

	// blockID is the block that the aggregated sig is signed for
	// sig is the aggregated signature data
	Unpack(blockID flow.Identifier, signerIDs []flow.Identifier, sigData []byte) (*BlockSignatureData, error)
}
