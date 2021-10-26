package hotstuff

import (
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
)

// RandomBeaconReconstructor collects signature shares, and reconstructs the
// group signature with enough shares.
type RandomBeaconReconstructor interface {
	// Verify verifies the signature under the stored public key corresponding to the signerID, and the stored message agreed about upfront.
	Verify(signerID flow.Identifier, sig crypto.Signature) error

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

// WeightedSignatureAggregator aggregates signatures of the same signature scheme and the same message from different signers.
// The public keys and message are aggreed upon upfront.
// It is also recommended to only aggregate signatures generated with keys representing equivalent security-bit level.
// The module is aware of weights assigned to each signer, as well as a total weight threshold.
// Implementation of SignatureAggregator must be concurrent safe.
type WeightedSignatureAggregator interface {
	// Verify verifies the signature under the stored public and message.
	//
	// The function errors:
	//  - engine.InvalidInputError if signerID is invalid (not a consensus participant)
	//  - module/signature.ErrInvalidFormat if signerID is valid but signature is cryptographically invalid
	//  - random error if the execution failed
	Verify(signerID flow.Identifier, sig crypto.Signature) error

	// TrustedAdd adds a signature to the internal set of signatures and adds the signer's
	// weight to the total collected weight, iff the signature is _not_ a duplicate.
	//
	// The total weight of all collected signatures (excluding duplicates) is returned regardless
	// of any returned error.
	// The function errors
	//  - engine.InvalidInputError if signerID is invalid (not a consensus participant)
	//  - engine.DuplicatedEntryError if the signer has been already added
	TrustedAdd(signerID flow.Identifier, sig crypto.Signature) (totalWeight uint64, exception error)

	// TotalWeight returns the total weight presented by the collected signatures.
	TotalWeight() uint64

	// Aggregate aggregates the signatures and returns the aggregated signature.
	//
	// Aggregate attempts to aggregate the internal signatures and returns the resulting signature data.
	// The function performs a final verification and errors if the aggregated signature is not valid. This is
	// required for the function safety since "TrustedAdd" allows adding invalid signatures.
	Aggregate() ([]flow.Identifier, []byte, error)
}

// BlockSignatureData is an intermediate struct for Packer to pack the
// aggregated signature data into raw bytes or unpack from raw bytes.
type BlockSignatureData struct {
	StakingSigners               []flow.Identifier
	RandomBeaconSigners          []flow.Identifier
	AggregatedStakingSig         []byte // if BLS is used, this is equivalent to crypto.Signature
	AggregatedRandomBeaconSig    []byte // if BLS is used, this is equivalent to crypto.Signature
	ReconstructedRandomBeaconSig crypto.Signature
}

// Packer packs aggregated signature data into raw bytes to be used in block header.
type Packer interface {
	// blockID is the block that the aggregated signature is for.
	// sig is the aggregated signature data.
	// Expected error returns during normal operations:
	//  * none; all errors are symptoms of inconsistent input data or corrupted internal state.
	Pack(blockID flow.Identifier, sig *BlockSignatureData) ([]flow.Identifier, []byte, error)

	// Unpack de-serializes the provided signature data.
	// blockID is the block that the aggregated sig is signed for
	// sig is the aggregated signature data
	// It returns:
	//  - (sigData, nil) if successfully unpacked the signature data
	//  - (nil, signature.ErrInvalidFormat) if failed to unpack the signature data
	Unpack(blockID flow.Identifier, signerIDs []flow.Identifier, sigData []byte) (*BlockSignatureData, error)
}
