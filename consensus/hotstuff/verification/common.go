package verification

import (
	"fmt"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/model/flow"
)

// messageFromParams generates the message we have to sign in order to be able
// to verify signatures without having the full block. To that effect, each datau
// structure that is signed contains the sometimes redundant view number and
// block ID; this allows us to create the signed message and verify the signed
// message without having the full block contents.
func messageFromParams(view uint64, blockID flow.Identifier) []byte {
	msg := flow.MakeID(struct {
		BlockID flow.Identifier
		View    uint64
	}{
		BlockID: blockID,
		View:    view,
	})
	return msg[:]
}

// checkVotesValidity checks the validity of each vote by checking that they are
// all for the same view number, the same block ID and that each vote is from a
// different signer.
func checkVotesValidity(votes []*model.Vote) error {

	// first, we should be sure to have votes at all
	if len(votes) == 0 {
		return fmt.Errorf("need at least one vote")
	}

	// we use this map to check each vote has a different signer
	signerIDs := make(map[flow.Identifier]struct{}, len(votes))

	// we use the view and block ID from the first vote to check that all votes
	// have the same view and bloc ID
	view := votes[0].View
	blockID := votes[0].BlockID

	// go through all votes to check their validity
	for _, vote := range votes {

		// if we have a view mismatch, bail
		if vote.View != view {
			return fmt.Errorf("view mismatch between votes (%d != %d)", vote.View, view)
		}

		// if we have a block ID mismatch, bail
		if vote.BlockID != blockID {
			return fmt.Errorf("block ID mismatch between votes (%x != %x)", vote.BlockID, blockID)
		}

		// register the signer in our map
		signerIDs[vote.SignerID] = struct{}{}
	}

	// check that we have as many signers as votes
	if len(signerIDs) != len(votes) {
		return fmt.Errorf("less signers than votes (signers: %d, votes: %d)", len(signerIDs), len(votes))
	}

	return nil
}
