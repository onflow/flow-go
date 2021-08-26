package votecollector

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// DoubleVoteDetector tracks double voting based on signerIDs.
// For each view new DoubleVoteDetector needs to be created.
// Tracks double voting for one view.
type DoubleVoteDetector struct {
	lock            sync.Mutex
	view            uint64
	votesBySignerID map[flow.Identifier]*model.Vote // signerID -> vote, track votes by signerID
}

func NewDoubleVoteDetector(view uint64) *DoubleVoteDetector {
	return &DoubleVoteDetector{
		view:            view,
		votesBySignerID: make(map[flow.Identifier]*model.Vote),
	}
}

// TrustedAdd adds vote to internal index and returns model.DoubleVoteError sentinel error if it's not the first vote.
// Votes should be validated before adding.
func (d *DoubleVoteDetector) TrustedAdd(vote *model.Vote) error {
	if vote.View != d.view {
		return fmt.Errorf("this DoubleVoteDetector tracks double voting for view %d but got vote for %d",
			d.view, vote.View)
	}

	d.lock.Lock()
	defer d.lock.Unlock()

	if firstVote, ok := d.votesBySignerID[vote.SignerID]; ok && vote.ID() != firstVote.ID() {
		return model.NewDoubleVoteErrorf(firstVote, vote, "double vote at view: %d", d.view)
	}

	// record vote as first one
	d.votesBySignerID[vote.SignerID] = vote

	return nil
}
