package hotstuff

import (
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// Verifier is the component responsible for the cryptographic integrity of
// votes, proposals and QC's against the block they are signing.
// Overall, there are two criteria for the validity of a vote and QC:
//  (1) the signer ID(s) must correspond to authorized consensus participants
//  (2) the signature must be cryptographically valid.
// Note that Verifier only implements (2). This API design allows to decouple
//  (i) the common logic for checking that a super-majority of the consensus
//      committee voted
// (ii) the handling of combined staking+RandomBeacon votes (consensus nodes)
//      vs only staking votes (collector nodes)
// On the one hand, this API design makes code less concise, as the two checks
// are now distributed over API boundaries. On the other hand, we can avoid
// repeated Identity lookups in the implementation, which increases performance.
type Verifier interface {

	// VerifyVote checks the validity of a vote for the given block.
	// The first return value indicates whether `sigData` is a valid signature
	// from the provided voter identity. It is the responsibility of the
	// calling code to ensure that `voter` is authorized to vote.
	// The implementation returns the following sentinel errors:
	// * signature.ErrInvalidFormat if the signature has an incompatible format.
	// * model.ErrInvalidSignature is the signature is invalid
	// * unexpected errors should be treated as symptoms of bugs or uncovered
	//   edge cases in the logic (i.e. as fatal)
	VerifyVote(voter *flow.Identity, sigData []byte, block *model.Block) error

	// VerifyQC checks the validity of a QC for the given block.
	// It returns:
	//  - `nil` to indicate the given `sigData` is a valid signature from the
	//          provided voter identity. It is the responsibility of the
	//          calling code to ensure that `voter` is authorized to vote,
	//          and the `voters` only contains authorized nodes (without duplicates).
	//  - `signature.ErrInvalidFormat` if the signature has an incompatible format.
	//  - error if running into any unexpected exception (i.e. fatal error)
	VerifyQC(voters flow.IdentityList, sigData []byte, block *model.Block) error
}
