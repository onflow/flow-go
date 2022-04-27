package hotstuff

import (
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
)

// RandomBeaconReconstructor encapsulates all methods needed by a Hotstuff leader to validate the
// beacon votes and reconstruct a beacon signature.
// The random beacon methods are based on a threshold signature scheme.
type RandomBeaconReconstructor interface {
	// Verify verifies the signature share under the signer's public key and the message agreed upon.
	// The function is thread-safe and wait-free (i.e. allowing arbitrary many routines to
	// execute the business logic, without interfering with each other).
	// It allows concurrent verification of the given signature.
	// Returns :
	//  - model.InvalidSignerError if signerIndex is invalid
	//  - model.ErrInvalidSignature if signerID is valid but signature is cryptographically invalid
	//  - other error if there is an unexpected exception.
	Verify(signerID flow.Identifier, sig crypto.Signature) error

	// TrustedAdd adds a share to the internal signature shares store.
	// There is no pre-check of the signature's validity _before_ adding it.
	// It is the caller's responsibility to make sure the signature was previously verified.
	// Nevertheless, the implementation guarantees safety (only correct threshold signatures
	// are returned) through a post-check (verifying the threshold signature
	// _after_ reconstruction before returning it).
	// The function is thread-safe but locks its internal state, thereby permitting only
	// one routine at a time to add a signature.
	// Returns:
	//  - (true, nil) if the signature has been added, and enough shares have been collected.
	//  - (false, nil) if the signature has been added, but not enough shares were collected.
	//  - (false, error) if there is any exception adding the signature share.
	//      - model.InvalidSignerError if signerIndex is invalid (out of the valid range)
	//  	- model.DuplicatedSignerError if the signer has been already added
	//      - other error if there is an unexpected exception.
	TrustedAdd(signerID flow.Identifier, sig crypto.Signature) (EnoughShares bool, err error)

	// EnoughShares indicates whether enough shares have been accumulated in order to reconstruct
	// a group signature. The function is thread-safe.
	EnoughShares() bool

	// Reconstruct reconstructs the group signature. The function is thread-safe but locks
	// its internal state, thereby permitting only one routine at a time.
	//
	// Returns:
	// - (signature, nil) if no error occurred
	// - (nil, model.InsufficientSignaturesError) if not enough shares were collected
	// - (nil, model.InvalidSignatureIncluded) if at least one collected share does not serialize to a valid BLS signature,
	//    or if the constructed signature failed to verify against the group public key and stored message. This post-verification
	//    is required  for safety, as `TrustedAdd` allows adding invalid signatures.
	// - (nil, error) for any other unexpected error.
	Reconstruct() (crypto.Signature, error)
}

// SigType is the aggregable signature type.
type SigType uint8

// SigType specifies the role of the signature in the protocol.
// Both types are aggregatable cryptographic signatures.
//  * SigTypeRandomBeacon type is for random beacon signatures.
//  * SigTypeStaking is for Hotstuff signatures.
const (
	SigTypeStaking SigType = iota
	SigTypeRandomBeacon
)

// Valid returns true if the signature is either SigTypeStaking or SigTypeRandomBeacon
// else return false
func (t SigType) Valid() bool {
	return t == SigTypeStaking || t == SigTypeRandomBeacon
}

// WeightedSignatureAggregator aggregates signatures of the same signature scheme and the
// same message from different signers. The public keys and message are agreed upon upfront.
// It is also recommended to only aggregate signatures generated with keys representing
// equivalent security-bit level.
// Furthermore, a weight [unsigned int64] is assigned to each signer ID. The
// WeightedSignatureAggregator internally tracks the total weight of all collected signatures.
// Implementations must be concurrency safe.
type WeightedSignatureAggregator interface {
	// Verify verifies the signature under the stored public keys and message.
	// Expected errors during normal operations:
	//  - model.InvalidSignerError if signerID is invalid (not a consensus participant)
	//  - model.ErrInvalidSignature if signerID is valid but signature is cryptographically invalid
	Verify(signerID flow.Identifier, sig crypto.Signature) error

	// TrustedAdd adds a signature to the internal set of signatures and adds the signer's
	// weight to the total collected weight, iff the signature is _not_ a duplicate. The
	// total weight of all collected signatures (excluding duplicates) is returned regardless
	// of any returned error.
	// Expected errors during normal operations:
	//  - model.InvalidSignerError if signerID is invalid (not a consensus participant)
	//  - model.DuplicatedSignerError if the signer has been already added
	TrustedAdd(signerID flow.Identifier, sig crypto.Signature) (totalWeight uint64, exception error)

	// TotalWeight returns the total weight presented by the collected signatures.
	TotalWeight() uint64

	// Aggregate aggregates the signatures and returns the aggregated signature.
	// The function performs a final verification and errors if the aggregated
	// signature is not valid. This is required for the function safety since
	// `TrustedAdd` allows adding invalid signatures.
	// Expected errors during normal operations:
	//  - model.InsufficientSignaturesError if no signatures have been added yet
	//  - model.InvalidSignatureIncludedError if some signature(s), included via TrustedAdd, are invalid
	Aggregate() ([]flow.Identifier, []byte, error)
}

// TimeoutSignatureAggregator aggregates timeout signatures of the same signature scheme but
// possibly distinct message from different signers. Each signer signs a message which is composed of
// view in which timeout has been created and highest QC view known to that replica.
// View and public keys are agreed upon upfront.
// It is also recommended to only aggregate signatures generated with keys representing
// equivalent security-bit level.
// Furthermore, a weight [unsigned int64] is assigned to each signer ID. The
// TimeoutSignatureAggregator internally tracks the total weight of all collected signatures.
// Implementations must be concurrency safe.
type TimeoutSignatureAggregator interface {
	// VerifyAndAdd verifies the signature under the stored public keys and adds signature with corresponding
	// highest QC view to the internal set. Internal set and collected weight is modified iff signature _is_ valid.
	// The total weight of all collected signatures (excluding duplicates) is returned regardless
	// of any returned error.
	// Expected errors during normal operations:
	//  - model.InvalidSignerError if signerID is invalid (not a consensus participant)
	//  - model.DuplicatedSignerError if the signer has been already added
	//  - model.ErrInvalidSignature if signerID is valid but signature is cryptographically invalid
	VerifyAndAdd(signerID flow.Identifier, sig crypto.Signature, highestQCView uint64) (totalWeight uint64, exception error)

	// TotalWeight returns the total weight presented by the collected signatures.
	TotalWeight() uint64

	// Aggregate aggregates the signatures and returns the aggregated signature.
	// The function performs a final verification of aggregated
	// signature. Caller can be sure that resulting signature is valid.
	// Expected errors during normal operations:
	//  - model.InsufficientSignaturesError if no signatures have been added yet
	Aggregate() (signers []flow.Identifier, highQCViews []uint64, aggregatedSignature crypto.Signature, exception error)
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
	// Pack serializes the provided BlockSignatureData into a precursor format of a QC.
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
	//  - (nil, model.ErrInvalidFormat) if failed to unpack the signature data
	Unpack(blockID flow.Identifier, signerIDs []flow.Identifier, sigData []byte) (*BlockSignatureData, error)
}
