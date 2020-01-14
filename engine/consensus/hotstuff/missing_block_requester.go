package hotstuff

import (
	"sync"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
)

type MissingBlockRequester struct {
	lock                  sync.RWMutex
	reqs                  map[string]types.BlockProposalRequest
	FetchedBlockProposals chan<- *types.BlockProposal
}

func (mbr *MissingBlockRequester) SetOnBlockFetchedHandler(handler func(*types.BlockProposal)) {
	panic("TODO")
}

func (mbr *MissingBlockRequester) FetchMissingBlock(view uint64, blockMRH []byte) {
	panic("TODO")
}
