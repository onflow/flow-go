package blockRequester

import "github.com/dapperlabs/flow-go/engine/consensus/componentsdef"

// BlockRequester requests block(s)
type BlockRequester struct {
	// Tracks the view of the QC which was used to initiate a block request
	OutstandingQCViews map[uint64]bool
}

// OnBlockRequest requests a batch of blocks inclusively between the view of
// `currentView`+1 and `QCView` and stores. If the range is large, it'll ask
// different sub-ranges from different peers
func (br *BlockRequester) OnBlockRequest(currentView uint64, QCView uint64) {
	br.OutstandingQCViews[QCView] = true
	// Request the blocks.
	panic("Implement me")
}

// OnIncorporatedBlock marks `block.View` as no longer outstanding.
// It's easier to mark it as false every time than to check if it's
// true, then mark it as false
// NOTE: doing `for x := range br.OutstandingQCViews` (getting the keys)
// should not show `block.View` if it's been made `false` for that view?
func (br *BlockRequester) OnIncorporatedBlock(block *def.Block) {
	br.OutstandingQCViews[block.View] = false
}
