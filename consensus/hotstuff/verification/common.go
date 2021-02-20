package verification

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
)

// makeVoteMessage generates the message we have to sign in order to be able
// to verify signatures without having the full block. To that effect, each data
// structure that is signed contains the sometimes redundant view number and
// block ID; this allows us to create the signed message and verify the signed
// message without having the full block contents.
func makeVoteMessage(view uint64, blockID flow.Identifier) []byte {
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
		// TODO: is checking the view and block id needed? (single votes are supposed to be already checked)

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

// stakingKeysAggregator is a structure that aggregates the staking
// public keys for QC verifications.
type stakingKeysAggregator struct {
	lastStakingSigners map[*flow.Identity]struct{}
	lastStakingKey     crypto.PublicKey
	lock               sync.Mutex
}

// creates a new staking keys aggregator
func newStakingKeysAggregator() *stakingKeysAggregator {
	aggregator := &stakingKeysAggregator{
		lastStakingSigners: map[*flow.Identity]struct{}{},
		lastStakingKey:     crypto.NeutralBLSPublicKey(),
		lock:               sync.Mutex{},
	}
	return aggregator
}

// aggregatedStakingKey returns the aggregated public of the input signers.
func (s *stakingKeysAggregator) aggregatedStakingKey(signers flow.IdentityList) (crypto.PublicKey, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	// this greedy algorithm assumes the signers set does not vary much from one call
	// to aggregatedStakingKey to another. It computes the delta of signers compared to the
	// latest list of signers and adjust the latest aggregated public key. This is faster
	// than aggregating the public keys from scratch at each call.
	//

	// get the signers delta and update the last list for the next comparison
	newSigners, missingSigners := s.identitiesDelta(signers)
	// add the new keys
	var err error
	s.lastStakingKey, err = crypto.AggregateBLSPublicKeys(
		append(newSigners.StakingKeys(), s.lastStakingKey))
	if err != nil {
		return nil, fmt.Errorf("adding new staking keys failed: %w", err)
	}
	// remove the missing keys
	s.lastStakingKey, err = crypto.RemoveBLSPublicKeys(s.lastStakingKey, missingSigners.StakingKeys())
	if err != nil {
		return nil, fmt.Errorf("removing missing staking keys failed: %w", err)
	}
	return s.lastStakingKey, nil
}

// identitiesDelta computes the delta between the reference s.lastStakingKey
// and the input identity list
func (s *stakingKeysAggregator) identitiesDelta(signers flow.IdentityList) (flow.IdentityList, flow.IdentityList) {
	var newSigners, missingSigners flow.IdentityList

	// create a map of the input list,
	// and check the new signers
	signersMap := map[*flow.Identity]struct{}{}
	for _, signer := range signers {
		signersMap[signer] = struct{}{}
		_, ok := s.lastStakingSigners[signer]
		if !ok {
			newSigners = append(newSigners, signer)
		}
	}

	// look for missing signers
	for signer := range s.lastStakingSigners {
		_, ok := signersMap[signer]
		if !ok {
			missingSigners = append(missingSigners, signer)
		}
	}
	// update the last signers for the next delta check
	s.lastStakingSigners = signersMap
	return newSigners, missingSigners
}
