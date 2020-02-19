package coldstuff

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff"
)

// Communicator is the interface for communication between nodes within Coldstuff.
//
// NOTE: It re-uses as much of the HotStuff interface and models as possible to
// simplify swapping between the two.
type Communicator interface {
	hotstuff.Communicator

	BroadcastCommit(commit *Commit) error
}
