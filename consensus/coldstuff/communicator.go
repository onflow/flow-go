package coldstuff

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff"
	model "github.com/dapperlabs/flow-go/model/coldstuff"
)

// Communicator is the interface for sending messages to other nodes within
// a ColdStuff consensus.
//
// NOTE: It re-uses as much of the HotStuff interface and models as possible to
// simplify swapping between the two.
type Communicator interface {
	hotstuff.Communicator

	BroadcastCommit(commit *model.Commit) error
}
