package hotstuff

import (
	"github.com/jrick/bitset"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
)

// SigAggregator provides abstraction of how vote signatures can be aggregated into an aggregated signature
// and how to verify a single signature and aggregated signature
type SigAggregator interface {
	// VerifySingleSig verifies a single signature
	VerifySingleSig(sig *types.VoteSignatureWithPubKey, message types.VoteBytes, view uint64) bool

	// Aggregate aggregates a slice of vote signatures into a single aggregated signature
	// TODO: what if the input is empty slice? should it return error?
	Aggregate(sigs []*types.VoteSignatureWithPubKey) types.AggregatedSignature

	// ReadSignerIndexes reads the indexes of all the signers whose siganture was aggregated into
	// the given aggregated signature
	ReadSignerIndexes(aggsig types.AggregatedSignature) bitset.Bytes

	// VerifyAggregatedSig checks if the given aggregated signature is from
	// aggsig: the aggregated signature
	// pubkeys: the public keys from the votes
	// view: the view of the block that was signed.
	// Note: view is needed in order to find the group public key for threshold siganture
	// TODO: is it possible the error can't be verified, in which case to return error
	VerifyAggregatedSig(aggsig types.AggregatedSignature, pubkeys []types.PubKey, message types.VoteBytes, view uint64) bool
}
