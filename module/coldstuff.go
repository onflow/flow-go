package module

import (
	model "github.com/dapperlabs/flow-go/model/coldstuff"
)

// ColdStuff is the interface for accepting proposals, votes, and commits to
// the ColdStuff consensus algorithm.
//
// NOTE: It re-uses as much of the HotStuff interface and models as possible to
// simplify swapping between the two.
type ColdStuff interface {
	HotStuff

	SubmitCommit(commit *model.Commit)
}
