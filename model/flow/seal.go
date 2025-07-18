package flow

import (
	"fmt"
)

// A Seal is produced when an Execution Result (referenced by `ResultID`) for
// particular block (referenced by `BlockID`) is committed into the chain.
// A Seal for a block B can be included in the payload B's descendants. Only
// in the respective fork where the seal for B is included, the referenced
// result is considered committed. Different forks might contain different
// seals for the same result (or in edge cases, even for different results).
//
// NOTES
// (1) As Seals are (currently) included in the payload, they are not strictly
// entities. (Entities can be sent between nodes as self-contained messages
// whose integrity is protected by a signature). By itself, a seal does
// _not_ contain enough information to determine its validity (verifier
// assignment cannot be computed) and its integrity is not protected by a
// signature of a node that is authorized to generate it. A seal should only
// be processed in the context of the block, which contains it.
//
// (2) Even though seals are not strictly entities, they still implement the
// Entity interface. This allows us to store and retrieve seals individually.
// CAUTION: As seals are part of the block payload, their _exact_ content must
// be preserved by the storage system. This includes the exact list of approval
// signatures (incl. order). While it is possible to construct different valid
// seals for the same result (using different subsets of assigned verifiers),
// they cannot be treated as equivalent for the following reason:
//
//   - Swapping a seal in a block with a different once changes the binary
//     representation of the block payload containing the seal.
//   - Changing the binary block representation would invalidate the block
//     proposer's signature.
//
// Therefore, to retrieve valid blocks from storage, it is required that
// the Seal.ID includes all fields with independent degrees of freedom
// (such as AggregatedApprovalSigs).
//
//structwrite:immutable - mutations allowed only within the constructor
type Seal struct {
	BlockID    Identifier
	ResultID   Identifier
	FinalState StateCommitment
	// AggregatedApprovalSigs can be nil/empty when verification is disabled or for the root seal.
	AggregatedApprovalSigs []AggregatedSignature // one AggregatedSignature per chunk
}

// UntrustedSeal is an untrusted input-only representation of a Seal,
// used for construction.
//
// This type exists to ensure that constructor functions are invoked explicitly
// with named fields, which improves clarity and reduces the risk of incorrect field
// ordering during construction.
//
// An instance of UntrustedSeal should be validated and converted into
// a trusted Seal using NewSeal constructor.
type UntrustedSeal Seal

// NewSeal creates a new instance of Seal.
// Construction Seal allowed only within the constructor.
//
// All errors indicate a valid Seal cannot be constructed from the input.
func NewSeal(untrusted UntrustedSeal) (*Seal, error) {
	if untrusted.BlockID == ZeroID {
		return nil, fmt.Errorf("block ID must not be zero")
	}
	if untrusted.ResultID == ZeroID {
		return nil, fmt.Errorf("result ID must not be zero")
	}
	if untrusted.FinalState == EmptyStateCommitment {
		return nil, fmt.Errorf("final state must not be empty")
	}
	return &Seal{
		BlockID:                untrusted.BlockID,
		ResultID:               untrusted.ResultID,
		FinalState:             untrusted.FinalState,
		AggregatedApprovalSigs: untrusted.AggregatedApprovalSigs,
	}, nil
}

func (s Seal) ID() Identifier {
	return MakeID(s)
}
