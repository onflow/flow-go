package voteaggregator

import (
	"fmt"
	"github.com/dapperlabs/flow-go/engine/consensus/HotStuff/types"
	"github.com/dapperlabs/flow-go/model/flow"
	protocol "github.com/dapperlabs/flow-go/protocol/badger"
	"github.com/dapperlabs/flow-go/utils/unittest"
	"github.com/dgraph-io/badger/v2"
	"math/rand"
	"testing"
)

const VOTE_SIZE int = 10
const THRESHOLD float32 = 0.66667
const STAKE uint64 = 10

type ViewParameter struct {
	thresholdStake float32
}

func (vp *ViewParameter) ThresholdState() float32 {
	return vp.thresholdStake
}

func TestVoteAggregator_Store(t *testing.T) {
}

func TestVoteAggregator_StoreVoteAndBuildQC(t *testing.T) {

}

func TestVoteAggregator_BuildQCOnReceivingBlock(t *testing.T) {
	va := &VoteAggregator{}
	votes := generateMockVotes(VOTE_SIZE)
	for _, vote := range votes {
		va.Store(vote, true)
	}

}

func TestVoteAggregator_PruneByView(t *testing.T) {

}

func generateMockVotes(size int) []*types.Vote {
	var votes []*types.Vote
	for i := 0; i < size; i++ {
		vote := &types.Vote{
			View:     uint64(i),
			BlockMRH: [32]byte{},
			Signature: &types.Signature{
				RawSignature: [32]byte{},
				SignerIdx:    0,
			},
		}
		rand.Read(vote.BlockMRH[:])
		votes = append(votes, vote)
	}

	return votes
}

func generateMockProtocolState(t *testing.T) protocol.State {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		state, _ := protocol.NewState(db)
		_ = state.Mutate().Bootstrap(flow.Genesis(ids))
	})
}

func generateMockConsensusIDs(size int) flow.IdentityList {
	var identities flow.IdentityList
	for i := 0; i < size; i++ {
		// creating mock identities as a random byte array
		var nodeID flow.Identifier
		rand.Read(nodeID[:])
		address := fmt.Sprintf("address%d", i)
		id := flow.Identity{
			NodeID:  nodeID,
			Address: address,
			Role:    flow.RoleConsensus,
			Stake:   STAKE,
		}
		identities = append(identities, id)
	}
	return identities
}
