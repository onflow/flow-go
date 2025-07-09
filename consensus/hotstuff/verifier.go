package hotstuff

import (
	"github.com/onflow/flow-go/model/flow"
)

// Verifier is the component responsible for the cryptographic integrity of
// votes, proposals and QC's against the block they are signing.
// Overall, there are two criteria for the validity of a vote and QC:
//
//	(1) the signer ID(s) must correspond to authorized consensus participants
//	(2) the signature must be cryptographically valid.
//
// Note that Verifier only implements (2). This API design allows to decouple
//
//	 (i)  the common logic for checking that a super-majority of the consensus
//	      committee voted
//	 (ii) the handling of combined staking+RandomBeacon votes (consensus nodes)
//		  vs only staking votes (collector nodes)
//
// On the one hand, this API design makes code less concise, as the two checks
// are now distributed over API boundaries. On the other hand, we can avoid
// repeated Identity lookups in the implementation, which increases performance.
type Verifier interface {

	// VerifyVote checks the cryptographic validity of a vote's `SigData` w.r.t.
	// the view and blockID. It is the responsibility of the calling code to ensure
	// that `voter` is authorized to vote.
	// Return values:
	//  * nil if `sigData` is cryptographically valid
	//  * model.InvalidFormatError if the signature has an incompatible format.
	//  * model.ErrInvalidSignature is the signature is invalid
	//  * model.InvalidSignerError is only relevant for extended signature schemes,
	//    where special signing authority is only given to a _subset_ of consensus
	//    participants (e.g. random beacon). In case a participant signed despite not
	//    being authorized, an InvalidSignerError is returned.
	//  * model.ErrViewForUnknownEpoch is only relevant for extended signature schemes,
	//    where querying of DKG might fail if no epoch containing the given view is known.
	//  * unexpected errors should be treated as symptoms of bugs or uncovered
	//    edge cases in the logic (i.e. as fatal)
	VerifyVote(voter *flow.IdentitySkeleton, sigData []byte, view uint64, blockID flow.Identifier) error

	// VerifyQC checks the cryptographic validity of a QC's `SigData` w.r.t. the
	// given view and blockID. It is the responsibility of the calling code to ensure that
	// all `signers` are authorized, without duplicates.
	// Return values:
	//  * nil if `sigData` is cryptographically valid
	//  * model.InvalidFormatError if `sigData` has an incompatible format
	//  * model.InsufficientSignaturesError if `signers is empty.
	//    Depending on the order of checks in the higher-level logic this error might
	//    be an indicator of a external byzantine input or an internal bug.
	//  * model.ErrInvalidSignature if a signature is invalid
	//  * model.InvalidSignerError is only relevant for extended signature schemes,
	//    where special signing authority is only given to a _subset_ of consensus
	//    participants (e.g. random beacon). In case a participant signed despite not
	//    being authorized, an InvalidSignerError is returned.
	//  * model.ErrViewForUnknownEpoch is only relevant for extended signature schemes,
	//    where querying of DKG might fail if no epoch containing the given view is known.
	//  * unexpected errors should be treated as symptoms of bugs or uncovered
	//	  edge cases in the logic (i.e. as fatal)
	VerifyQC(signers flow.IdentitySkeletonList, sigData []byte, view uint64, blockID flow.Identifier) error

	// VerifyTC checks cryptographic validity of the TC's `sigData` w.r.t. the
	// given view. It is the responsibility of the calling code to ensure
	// that all `signers` are authorized, without duplicates. Return values:
	//  * nil if `sigData` is cryptographically valid
	//  * model.InsufficientSignaturesError if `signers is empty.
	//  * model.InvalidFormatError if `signers`/`highQCViews` have differing lengths
	//  * model.ErrInvalidSignature if a signature is invalid
	//  * unexpected errors should be treated as symptoms of bugs or uncovered
	//	  edge cases in the logic (i.e. as fatal)
	VerifyTC(signers flow.IdentitySkeletonList, sigData []byte, view uint64, highQCViews []uint64) error
}
