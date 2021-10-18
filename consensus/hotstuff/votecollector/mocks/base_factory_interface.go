package mocks

import (
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
)

// BaseFactory specifies an interface with the same API as
// `votecollector.baseFactory`. Thereby, we can generate mocks
// with the same API convention
type BaseFactory interface {
	Create(block *model.Block) (hotstuff.VerifyingVoteProcessor, error)
}
