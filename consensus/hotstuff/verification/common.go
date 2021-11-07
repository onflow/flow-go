package verification

import (
	"fmt"
	"sync/atomic"
	"unsafe"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
)

// MakeVoteMessage generates the message we have to sign in order to be able
// to verify signatures without having the full block. To that effect, each data
// structure that is signed contains the sometimes redundant view number and
// block ID; this allows us to create the signed message and verify the signed
// message without having the full block contents.
func MakeVoteMessage(view uint64, blockID flow.Identifier) []byte {
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

type aggregate struct {
	lastStakingSigners map[flow.Identifier]*flow.Identity
	lastStakingKey     crypto.PublicKey
}

// stakingKeysAggregator is a structure that aggregates the staking
// public keys for QC verifications.
type stakingKeysAggregator struct {
	current unsafe.Pointer // *aggregate type
	inner   *aggregate
}

// creates a new staking keys aggregator
func newStakingKeysAggregator() *stakingKeysAggregator {
	aggregator := &stakingKeysAggregator{}

	aggregator.inner = &aggregate{
		lastStakingSigners: map[flow.Identifier]*flow.Identity{},
		lastStakingKey:     NeutralBLSPublicKey(),
	}

	aggregator.current = unsafe.Pointer(aggregator.inner)

	return aggregator
}

// use this function to obtain the current config
func (s *stakingKeysAggregator) getCurrent() *aggregate {
	return (*aggregate)(atomic.LoadPointer(&s.current))
}

// periodically sets aggregate struct as current
func (s *stakingKeysAggregator) updateCurrent(agg *aggregate) {
	atomic.StorePointer(&s.current, unsafe.Pointer(agg))
}

// aggregatedStakingKey returns the aggregated public key of the input signers.
func (s *stakingKeysAggregator) aggregatedStakingKey(signers flow.IdentityList) (crypto.PublicKey, error) {

	// this greedy algorithm assumes the signers set does not vary much from one call
	// to aggregatedStakingKey to another. It computes the delta of signers compared to the
	// latest list of signers and adjust the latest aggregated public key. This is faster
	// than aggregating the public keys from scratch at each call.
	// Considering votes have equal weights, aggregating keys from scratch takes n*2/3 key operations, where
	// n is the number of Hotstuff participants. This corresponds to the worst case of the greedy
	// algorithm. The worst case happens when the 2/3 latest signers and the 2/3 new signers only
	// have 1/3 in common (the minimum common ratio).

	inner := s.getCurrent()

	lastSet := inner.lastStakingSigners
	lastKey := inner.lastStakingKey

	// get the signers delta and update the last list for the next comparison
	newSignerKeys, missingSignerKeys, updatedSignerSet := identitiesDeltaKeys(signers, lastSet)
	// add the new keys
	var err error
	updatedKey, err := AggregateBLSPublicKeys(append(newSignerKeys, lastKey))
	if err != nil {
		return nil, fmt.Errorf("adding new staking keys failed: %w", err)
	}
	// remove the missing keys
	updatedKey, err = RemoveBLSPublicKeys(updatedKey, missingSignerKeys)
	if err != nil {
		return nil, fmt.Errorf("removing missing staking keys failed: %w", err)
	}

	// update the latest list and public key. The current thread may overwrite the result of another thread
	// but the greedy algorithm remains valid.

	// create a new inner struct to hold the data, in a thread-safe way
	nextInner := &aggregate{
		lastStakingSigners: updatedSignerSet,
		lastStakingKey:     updatedKey,
	}

	// swap the struct out
	s.updateCurrent(nextInner)

	return updatedKey, nil
}

// identitiesDeltaKeys computes the delta between the reference s.lastStakingSigners
// and the input identity list.
// It returns a list of the new signer keys, a list of the missing signer keys and the new map of signers.
func identitiesDeltaKeys(signers flow.IdentityList, lastSet map[flow.Identifier]*flow.Identity) (
	[]crypto.PublicKey, []crypto.PublicKey, map[flow.Identifier]*flow.Identity) {

	var newSignerKeys, missingSignerKeys []crypto.PublicKey

	// create a map of the input list,
	// and check the new signers
	signersMap := map[flow.Identifier]*flow.Identity{}
	for _, signer := range signers {
		signersMap[signer.ID()] = signer
		_, ok := lastSet[signer.ID()]
		if !ok {
			newSignerKeys = append(newSignerKeys, signer.StakingPubKey)
		}
	}

	// look for missing signers
	for signerID, signer := range lastSet {
		_, ok := signersMap[signerID]
		if !ok {
			missingSignerKeys = append(missingSignerKeys, signer.StakingPubKey)
		}
	}
	return newSignerKeys, missingSignerKeys, signersMap
}
