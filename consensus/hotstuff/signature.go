package hotstuff

import (
	"github.com/onflow/crypto"

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
	// The function performs a final verification and errors if the aggregated signature is invalid. This is
	// required for the function safety since `TrustedAdd` allows adding invalid signatures.
	// The function errors with:
	//   - model.InsufficientSignaturesError if no signatures have been added yet
	//   - model.InvalidSignatureIncludedError if:
	//     -- some signature(s), included via TrustedAdd, fail to deserialize (regardless of the aggregated public key)
	//     -- or all signatures deserialize correctly but some signature(s), included via TrustedAdd, are
	//       invalid (while aggregated public key is valid)
	//   - model.InvalidAggregatedKeyError if all signatures deserialize correctly but the signer's
	//     staking public keys sum up to an invalid key (BLS identity public key).
	//     Any aggregated signature would fail the cryptographic verification under the identity public
	//     key and therefore such signature is considered invalid. Such scenario can only happen if
	//     staking public keys of signers were forged to add up to the identity public key.
	//     Under the assumption that all staking key PoPs are valid, this error case can only
	//     happen if all signers are malicious and colluding. If there is at least one honest signer,
	//     there is a negligible probability that the aggregated key is identity.
	//
	// The function is thread-safe.
	Aggregate() (flow.IdentifierList, []byte, error)
}

// TimeoutSignatureAggregator aggregates timeout signatures for one particular view.
// When instantiating a TimeoutSignatureAggregator, the following information is supplied:
//   - The view for which the aggregator collects timeouts.
//   - For each replicas that is authorized to send a timeout at this particular view:
//     the node ID, public staking keys, and weight
//
// Timeouts for other views or from non-authorized replicas are rejected.
// In their TimeoutObjects, replicas include a signature over the pair (view, newestQCView),
// where `view` is the view number the timeout is for and `newestQCView` is the view of
// the newest QC known to the replica. TimeoutSignatureAggregator collects these signatures,
// internally tracks the total weight of all collected signatures. Note that in general the
// signed messages are different, which makes the aggregation a comparatively expensive operation.
// Upon calling `Aggregate`, the TimeoutSignatureAggregator aggregates all valid signatures collected
// up to this point. The aggregate signature is guaranteed to be correct, as only valid signatures
// are excepted as inputs.
// TimeoutSignatureAggregator internally tracks the total weight of all collected signatures.
// Implementations must be concurrency safe.
type TimeoutSignatureAggregator interface {
	// VerifyAndAdd verifies the signature under the stored public keys and adds the signature and the corresponding
	// highest QC to the internal set. Internal set and collected weight is modified iff signature _is_ valid.
	// The total weight of all collected signatures (excluding duplicates) is returned regardless
	// of any returned error.
	// Expected errors during normal operations:
	//  - model.InvalidSignerError if signerID is invalid (not a consensus participant)
	//  - model.DuplicatedSignerError if the signer has been already added
	//  - model.ErrInvalidSignature if signerID is valid but signature is cryptographically invalid
	VerifyAndAdd(signerID flow.Identifier, sig crypto.Signature, newestQCView uint64) (totalWeight uint64, exception error)

	// TotalWeight returns the total weight presented by the collected signatures.
	TotalWeight() uint64

	// View returns the view that this instance is aggregating signatures for.
	View() uint64

	// Aggregate aggregates the signatures and returns with additional data.
	// Aggregated signature will be returned as SigData of timeout certificate.
	// Caller can be sure that resulting signature is valid.
	// Expected errors during normal operations:
	//  - model.InsufficientSignaturesError if no signatures have been added yet
	Aggregate() (signersInfo []TimeoutSignerInfo, aggregatedSig crypto.Signature, exception error)
}

// TimeoutSignerInfo is a helper structure that stores the QC views that each signer
// contributed to a TC. Used as result of TimeoutSignatureAggregator.Aggregate()
type TimeoutSignerInfo struct {
	NewestQCView uint64
	Signer       flow.Identifier
}

// BlockSignatureData is an intermediate struct for Packer to pack the
// aggregated signature data into raw bytes or unpack from raw bytes.
type BlockSignatureData struct {
	StakingSigners               flow.IdentifierList
	RandomBeaconSigners          flow.IdentifierList
	AggregatedStakingSig         []byte // if BLS is used, this is equivalent to crypto.Signature
	AggregatedRandomBeaconSig    []byte // if BLS is used, this is equivalent to crypto.Signature
	ReconstructedRandomBeaconSig crypto.Signature
}

// Packer packs aggregated signature data into raw bytes to be used in block header.
type Packer interface {
	// Pack serializes the provided BlockSignatureData into a precursor format of a QC.
	// view is the view of the block that the aggregated signature is for.
	// sig is the aggregated signature data.
	// Expected error returns during normal operations:
	//  * none; all errors are symptoms of inconsistent input data or corrupted internal state.
	Pack(view uint64, sig *BlockSignatureData) (signerIndices []byte, sigData []byte, err error)

	// Unpack de-serializes the provided signature data.
	// sig is the aggregated signature data
	// It returns:
	//  - (sigData, nil) if successfully unpacked the signature data
	//  - (nil, model.InvalidFormatError) if failed to unpack the signature data
	Unpack(signerIdentities flow.IdentityList, sigData []byte) (*BlockSignatureData, error)
}
