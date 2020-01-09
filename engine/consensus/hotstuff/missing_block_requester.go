package hotstuff

import (
	"sync"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
)

type MissingBlockRequester struct {
	lock                  sync.RWMutex
	reqs                  map[types.MRH]types.BlockProposalRequest
	FetchedBlockProposals chan<- *types.BlockProposal
}

func (mbr *MissingBlockRequester) SetOnBlockFetchedHandler(handler func(*types.BlockProposal)) {
	panic("TODO")
}

func (mbr *MissingBlockRequester) FetchMissingBlock(view uint64, blockMRH types.MRH) {
	panic("TODO")
}
