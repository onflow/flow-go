package forkchoice

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/forks"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/forks/finalizer"
	mockdist "github.com/dapperlabs/flow-go/engine/consensus/hotstuff/notifications/mock"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/hotstuff"
	mockfinalizer "github.com/dapperlabs/flow-go/module/mock"
)

const VALIDATOR_SIZE uint32 = 7

type ViewPair struct {
	qcView    uint64
	blockView uint64
}

type Blocks struct {
	blockMap  map[uint64]*hotstuff.Block
	blockList []*hotstuff.Block
}

type QCs struct {
	qcMap  map[uint64]*hotstuff.QuorumCertificate
	qcList []*hotstuff.QuorumCertificate
}

// HAPPY PATH
// As the leader of 6, if receives [1, 2], [2, 3], [3, 4], [4, 5], then the receive a QC for 5
// the fork choice should return block and qc for 5
func TestHappyPath(t *testing.T) {
	curView := uint64(6)

	fc, finCallback, notifier, root := initNewestForkChoice(t, 1) // includes genesis block (v1)

	p1 := ViewPair{1, 2}
	p2 := ViewPair{2, 3}
	p3 := ViewPair{3, 4}
	p4 := ViewPair{4, 5}

	blocks := generateBlocks(root.QC, p1, p2, p3, p4)
	finalizedView := uint64(2)

	for _, block := range blocks.blockList {
		notifier.On("OnBlockIncorporated", block).Return().Once()
		if block.View == finalizedView {
			notifier.On("OnFinalizedBlock", block).Return().Once()
			finCallback.On("MakeFinal", block.BlockID).Return(nil).Once()
		}
		err := fc.finalizer.AddBlock(block)
		require.NoError(t, err)
	}

	prefferedQC := makeQC(5, blocks.blockMap[5].BlockID)
	notifier.On("OnQcIncorporated", prefferedQC).Return().Once()
	err := fc.AddQC(prefferedQC)
	require.NoError(t, err)

	// the fork choice should return block and qc for view 5
	notifier.On("OnForkChoiceGenerated", curView, prefferedQC).Return().Once()
	choiceBlock, choiceQC, err := fc.MakeForkChoice(curView)
	require.Equal(t, prefferedQC, choiceQC)
	require.Equal(t, blocks.blockMap[choiceQC.View], choiceBlock)
	require.NoError(t, err)
	notifier.AssertExpectations(t)
}

// NEWEST OVER LONGEST
// As the leader of 6, if receives [1,2], [2,3], [3,4], [4,5], and preferredQC is for view 4
// the fork choice should return block and qc for 4
func TestNewestOverLongest1(t *testing.T) {
	curView := uint64(6)

	fc, finCallback, notifier, root := initNewestForkChoice(t, 1) // includes genesis block (v1)

	p1 := ViewPair{1, 2}
	p2 := ViewPair{2, 3}
	p3 := ViewPair{3, 4}
	p4 := ViewPair{4, 5}

	blocks := generateBlocks(root.QC, p1, p2, p3, p4)
	finalizedView := uint64(2)

	for _, block := range blocks.blockList {
		notifier.On("OnBlockIncorporated", block).Return().Once()
		if block.View == finalizedView {
			notifier.On("OnFinalizedBlock", block).Return().Once()
			finCallback.On("MakeFinal", block.BlockID).Return(nil).Once()
		}
		err := fc.finalizer.AddBlock(block)
		require.NoError(t, err)
	}

	prefferedQC := makeQC(4, blocks.blockMap[4].BlockID)
	notifier.On("OnQcIncorporated", prefferedQC).Return().Once()
	err := fc.AddQC(prefferedQC)
	require.NoError(t, err)

	// the fork choice should return block and qc for view 5
	notifier.On("OnForkChoiceGenerated", curView, prefferedQC).Return().Once()
	choiceBlock, choiceQC, err := fc.MakeForkChoice(curView)
	require.Equal(t, prefferedQC, choiceQC)
	require.Equal(t, blocks.blockMap[choiceQC.View], choiceBlock)
	require.NoError(t, err)
	notifier.AssertExpectations(t)
}

// NEWEST OVER LONGEST
// As the leader of 7, if receives [1,2], [2,3], [3,4], [4,5], [2,6], and preferredQC is 6
// the fork choice should return block and qc for 6
func TestNewestOverLongest2(t *testing.T) {
	curView := uint64(7)

	fc, finCallback, notifier, root := initNewestForkChoice(t, 1) // includes genesis block (v1)

	p1 := ViewPair{1, 2}
	p2 := ViewPair{2, 3}
	p3 := ViewPair{3, 4}
	p4 := ViewPair{4, 5}
	p5 := ViewPair{2, 6}

	blocks := generateBlocks(root.QC, p1, p2, p3, p4, p5)
	finalizedView := uint64(2)

	for _, block := range blocks.blockList {
		notifier.On("OnBlockIncorporated", block).Return().Once()
		if block.View == finalizedView {
			notifier.On("OnFinalizedBlock", block).Return().Once()
			finCallback.On("MakeFinal", block.BlockID).Return(nil).Once()
		}
		err := fc.finalizer.AddBlock(block)
		require.NoError(t, err)
	}

	prefferedQC := makeQC(6, blocks.blockMap[6].BlockID)
	notifier.On("OnQcIncorporated", prefferedQC).Return().Once()
	err := fc.AddQC(prefferedQC)
	require.NoError(t, err)

	// the fork choice should return block and qc for view 5
	notifier.On("OnForkChoiceGenerated", curView, prefferedQC).Return().Once()
	choiceBlock, choiceQC, err := fc.MakeForkChoice(curView)
	require.Equal(t, prefferedQC, choiceQC)
	require.Equal(t, blocks.blockMap[choiceQC.View], choiceBlock)
	require.NoError(t, err)
	notifier.AssertExpectations(t)
}

// NEWEST OVER LONGEST
// As the leader of 7, if receives [1,2], [2,3], [3,4], [4,5], [2,6], and preferredQC is 4
// the fork choice should return block and qc for 4
func TestNewestOverLongest3(t *testing.T) {
	curView := uint64(7)

	fc, finCallback, notifier, root := initNewestForkChoice(t, 1) // includes genesis block (v1)

	p1 := ViewPair{1, 2}
	p2 := ViewPair{2, 3}
	p3 := ViewPair{3, 4}
	p4 := ViewPair{4, 5}
	p5 := ViewPair{2, 6}

	blocks := generateBlocks(root.QC, p1, p2, p3, p4, p5)
	finalizedView := uint64(2)

	for _, block := range blocks.blockList {
		notifier.On("OnBlockIncorporated", block).Return().Once()
		if block.View == finalizedView {
			notifier.On("OnFinalizedBlock", block).Return().Once()
			finCallback.On("MakeFinal", block.BlockID).Return(nil).Once()
		}
		err := fc.finalizer.AddBlock(block)
		require.NoError(t, err)
	}

	prefferedQC := makeQC(4, blocks.blockMap[4].BlockID)
	notifier.On("OnQcIncorporated", prefferedQC).Return().Once()
	err := fc.AddQC(prefferedQC)
	require.NoError(t, err)

	// the fork choice should return block and qc for view 5
	notifier.On("OnForkChoiceGenerated", curView, prefferedQC).Return().Once()
	choiceBlock, choiceQC, err := fc.MakeForkChoice(curView)
	require.Equal(t, prefferedQC, choiceQC)
	require.Equal(t, blocks.blockMap[choiceQC.View], choiceBlock)
	require.NoError(t, err)
	notifier.AssertExpectations(t)
}

// FORK BELOW LOCKED
// As the leader of 8, if receives [1,2], [2,3], [3,4], [4,5], [2,6], [6,7], and preferredQC is 6
// the fork choice should return block and qc for 6
func TestForkBelowLocked1(t *testing.T) {
	curView := uint64(8)

	fc, finCallback, notifier, root := initNewestForkChoice(t, 1) // includes genesis block (v1)

	p1 := ViewPair{1, 2}
	p2 := ViewPair{2, 3}
	p3 := ViewPair{3, 4}
	p4 := ViewPair{4, 5}
	p5 := ViewPair{2, 6}
	p6 := ViewPair{6, 7}

	blocks := generateBlocks(root.QC, p1, p2, p3, p4, p5, p6)
	finalizedView := uint64(2)

	for _, block := range blocks.blockList {
		notifier.On("OnBlockIncorporated", block).Return().Once()
		if block.View == finalizedView {
			notifier.On("OnFinalizedBlock", block).Return().Once()
			finCallback.On("MakeFinal", block.BlockID).Return(nil).Once()
		}
		err := fc.finalizer.AddBlock(block)
		require.NoError(t, err)
	}

	prefferedQC := makeQC(6, blocks.blockMap[6].BlockID)
	notifier.On("OnQcIncorporated", prefferedQC).Return().Once()
	err := fc.AddQC(prefferedQC)
	require.NoError(t, err)

	// the fork choice should return block and qc for view 5
	notifier.On("OnForkChoiceGenerated", curView, prefferedQC).Return().Once()
	choiceBlock, choiceQC, err := fc.MakeForkChoice(curView)
	require.Equal(t, prefferedQC, choiceQC)
	require.Equal(t, blocks.blockMap[choiceQC.View], choiceBlock)
	require.NoError(t, err)
	notifier.AssertExpectations(t)
}

// FORK BELOW LOCKED
// As the leader of 8, if receives [1,2], [2,3], [3,4], [4,5], [2,6], [6,7], and preferredQC is 7
// the fork choice should return block and qc for 7
func TestForkBelowLocked2(t *testing.T) {
	curView := uint64(8)

	fc, finCallback, notifier, root := initNewestForkChoice(t, 1) // includes genesis block (v1)

	p1 := ViewPair{1, 2}
	p2 := ViewPair{2, 3}
	p3 := ViewPair{3, 4}
	p4 := ViewPair{4, 5}
	p5 := ViewPair{2, 6}
	p6 := ViewPair{6, 7}

	blocks := generateBlocks(root.QC, p1, p2, p3, p4, p5, p6)
	finalizedView := uint64(2)

	for _, block := range blocks.blockList {
		notifier.On("OnBlockIncorporated", block).Return().Once()
		if block.View == finalizedView {
			notifier.On("OnFinalizedBlock", block).Return().Once()
			finCallback.On("MakeFinal", block.BlockID).Return(nil).Once()
		}
		err := fc.finalizer.AddBlock(block)
		require.NoError(t, err)
	}

	prefferedQC := makeQC(7, blocks.blockMap[7].BlockID)
	notifier.On("OnQcIncorporated", prefferedQC).Return().Once()
	err := fc.AddQC(prefferedQC)
	require.NoError(t, err)

	// the fork choice should return block and qc for view 5
	notifier.On("OnForkChoiceGenerated", curView, prefferedQC).Return().Once()
	choiceBlock, choiceQC, err := fc.MakeForkChoice(curView)
	require.Equal(t, prefferedQC, choiceQC)
	require.Equal(t, blocks.blockMap[choiceQC.View], choiceBlock)
	require.NoError(t, err)
	notifier.AssertExpectations(t)
}

// FORK BELOW LOCKED
// As the leader of 9, if receives [1,2], [2,3], [3,4], [4,5], [2,6], [6,7], [7,8], and preferredQC is 8
// the fork choice should return block and qc for 8
func TestForkBelowLocked3(t *testing.T) {
	curView := uint64(9)

	fc, finCallback, notifier, root := initNewestForkChoice(t, 1) // includes genesis block (v1)

	p1 := ViewPair{1, 2}
	p2 := ViewPair{2, 3}
	p3 := ViewPair{3, 4}
	p4 := ViewPair{4, 5}
	p5 := ViewPair{2, 6}
	p6 := ViewPair{6, 7}
	p7 := ViewPair{7, 8}

	blocks := generateBlocks(root.QC, p1, p2, p3, p4, p5, p6, p7)
	finalizedView := uint64(2)

	for _, block := range blocks.blockList {
		notifier.On("OnBlockIncorporated", block).Return().Once()
		if block.View == finalizedView {
			notifier.On("OnFinalizedBlock", block).Return().Once()
			finCallback.On("MakeFinal", block.BlockID).Return(nil).Once()
		}
		err := fc.finalizer.AddBlock(block)
		require.NoError(t, err)
	}

	prefferedQC := makeQC(8, blocks.blockMap[8].BlockID)
	notifier.On("OnQcIncorporated", prefferedQC).Return().Once()
	err := fc.AddQC(prefferedQC)
	require.NoError(t, err)

	// the fork choice should return block and qc for view 5
	notifier.On("OnForkChoiceGenerated", curView, prefferedQC).Return().Once()
	choiceBlock, choiceQC, err := fc.MakeForkChoice(curView)
	require.Equal(t, prefferedQC, choiceQC)
	require.Equal(t, blocks.blockMap[choiceQC.View], choiceBlock)
	require.NoError(t, err)
	notifier.AssertExpectations(t)
}

// FORK BELOW LOCKED
// As the leader of 9, if receives [1,2], [2,3], [3,4], [4,5], [2,6], [6,7], [7,8], and preferredQC is 7
// the fork choice should return block and qc for 7
func TestForkBelowLocked4(t *testing.T) {
	curView := uint64(9)

	fc, finCallback, notifier, root := initNewestForkChoice(t, 1) // includes genesis block (v1)

	p1 := ViewPair{1, 2}
	p2 := ViewPair{2, 3}
	p3 := ViewPair{3, 4}
	p4 := ViewPair{4, 5}
	p5 := ViewPair{2, 6}
	p6 := ViewPair{6, 7}
	p7 := ViewPair{7, 8}

	blocks := generateBlocks(root.QC, p1, p2, p3, p4, p5, p6, p7)
	finalizedView := uint64(2)

	for _, block := range blocks.blockList {
		notifier.On("OnBlockIncorporated", block).Return().Once()
		if block.View == finalizedView {
			notifier.On("OnFinalizedBlock", block).Return().Once()
			finCallback.On("MakeFinal", block.BlockID).Return(nil).Once()
		}
		err := fc.finalizer.AddBlock(block)
		require.NoError(t, err)
	}

	prefferedQC := makeQC(7, blocks.blockMap[7].BlockID)
	notifier.On("OnQcIncorporated", prefferedQC).Return().Once()
	err := fc.AddQC(prefferedQC)
	require.NoError(t, err)

	// the fork choice should return block and qc for view 5
	notifier.On("OnForkChoiceGenerated", curView, prefferedQC).Return().Once()
	choiceBlock, choiceQC, err := fc.MakeForkChoice(curView)
	require.Equal(t, prefferedQC, choiceQC)
	require.Equal(t, blocks.blockMap[choiceQC.View], choiceBlock)
	require.NoError(t, err)
	notifier.AssertExpectations(t)
}

func generateBlocks(rootQC *hotstuff.QuorumCertificate, viewPairs ...ViewPair) *Blocks {
	blocks := &Blocks{
		blockMap: make(map[uint64]*hotstuff.Block),
	}
	qcs := &QCs{
		qcMap: make(map[uint64]*hotstuff.QuorumCertificate),
	}
	var lastBlockView uint64
	for _, viewPair := range viewPairs {
		var qc *hotstuff.QuorumCertificate
		if viewPair.qcView == 1 {
			qc = rootQC
		} else {
			existedQc, exists := qcs.qcMap[viewPair.qcView]
			if !exists {
				qc = makeQC(viewPair.qcView, blocks.blockMap[lastBlockView].BlockID)
			} else {
				qc = existedQc
			}
		}
		qcs.AddQC(qc)

		block := makeBlock(viewPair.blockView, qcs.qcMap[viewPair.qcView], flow.ZeroID)
		lastBlockView = block.View
		blocks.AddBlock(block)
	}

	return blocks
}

func initNewestForkChoice(t *testing.T, view uint64) (*NewestForkChoice, *mockfinalizer.Finalizer, *mockdist.Consumer, *forks.BlockQC) {
	notifier := &mockdist.Consumer{}
	finalizationCallback := &mockfinalizer.Finalizer{}

	// construct Finalizer
	root := makeRootBlock(t, view)
	notifier.On("OnBlockIncorporated", root.Block).Return().Once()
	fnlzr, _ := finalizer.New(root, finalizationCallback, notifier)

	// construct ForkChoice
	notifier.On("OnQcIncorporated", root.QC).Return().Once()
	fc, _ := NewNewestForkChoice(fnlzr, notifier)

	return fc, finalizationCallback, notifier, root
}

func makeQC(view uint64, blockID flow.Identifier) *hotstuff.QuorumCertificate {
	return &hotstuff.QuorumCertificate{
		View:    view,
		BlockID: blockID,
	}
}

func (b *Blocks) AddBlock(block *hotstuff.Block) {
	b.blockMap[block.View] = block
	b.blockList = append(b.blockList, block)
}

func (q *QCs) AddQC(qc *hotstuff.QuorumCertificate) {
	q.qcMap[qc.View] = qc
	q.qcList = append(q.qcList, qc)
}
