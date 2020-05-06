package forkchoice

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	hs "github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/forks"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/forks/finalizer"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/mocks"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/model/flow"
	mockfinalizer "github.com/dapperlabs/flow-go/module/mock"
)

type ViewPair struct {
	qcView    uint64
	blockView uint64
}

type Blocks struct {
	blockMap  map[uint64]*model.Block
	blockList []*model.Block
}

type QCs struct {
	qcMap  map[uint64]*model.QuorumCertificate
	qcList []*model.QuorumCertificate
}

// NOTATION:
// [a, b] is a block at view "b" with a QC with view "a",
// e.g., [1, 2] means a block at view "2" with an included  QC for view "1"

// HAPPY PATH
// As the leader of 6: we receive [1, 2], [2, 3], [3, 4], [4, 5] and receive enough votes to build QC for block 5.
// the fork choice should return block and qc for 5
func TestHappyPath(t *testing.T) {
	curView := uint64(6)

	f, notifier, root := initNewestForkChoice(t, 1) // includes genesis block (v1)

	p1 := ViewPair{1, 2}
	p2 := ViewPair{2, 3}
	p3 := ViewPair{3, 4}
	p4 := ViewPair{4, 5}

	blocks := generateBlocks(root.QC, p1, p2, p3, p4)
	for _, block := range blocks.blockList {
		err := f.AddBlock(block)
		require.NoError(t, err)
	}

	preferedQC := makeQC(5, blocks.blockMap[5].BlockID)
	err := f.AddQC(preferedQC)
	require.NoError(t, err)

	// the fork choice should return block and qc for view 5
	choiceQC, choiceBlock, err := f.MakeForkChoice(curView)
	require.NoError(t, err)
	require.Equal(t, preferedQC, choiceQC)
	require.Equal(t, blocks.blockMap[choiceQC.View], choiceBlock)
	notifier.AssertCalled(t, "OnForkChoiceGenerated", curView, preferedQC)
}

// NOT ENOUGH VOTES TO BUILD QC FOR LATEST FORK: fork with newest QC
// As the leader of 6: we receive [1,2], [2,3], [3,4], [4,5] but we don't receive enough votes to build a qc for block 5.
// the fork choice should return block and qc for 4
func TestNoQcForLatestMainFork(t *testing.T) {
	curView := uint64(6)

	f, notifier, root := initNewestForkChoice(t, 1) // includes genesis block (v1)

	p1 := ViewPair{1, 2}
	p2 := ViewPair{2, 3}
	p3 := ViewPair{3, 4}
	p4 := ViewPair{4, 5}

	blocks := generateBlocks(root.QC, p1, p2, p3, p4)
	for _, block := range blocks.blockList {
		err := f.AddBlock(block)
		require.NoError(t, err)
	}

	// the fork choice should return block and qc for view 5
	preferedQC := makeQC(4, blocks.blockMap[4].BlockID)
	choiceQC, choiceBlock, err := f.MakeForkChoice(curView)
	require.NoError(t, err)
	require.Equal(t, preferedQC, choiceQC)
	require.Equal(t, blocks.blockMap[preferedQC.View], choiceBlock)
	notifier.AssertCalled(t, "OnForkChoiceGenerated", curView, preferedQC)
}

// NEWEST OVER LONGEST: HAPPY PATH: extend newest
// As the leader of 7: we receive [1,2], [2,3], [3,4], [4,5], [2,6], and receive enough votes to build QC for block 6.
// the fork choice should return block and qc for 6
func TestNewestOverLongestHappyPath(t *testing.T) {
	curView := uint64(7)

	f, notifier, root := initNewestForkChoice(t, 1) // includes genesis block (v1)

	p1 := ViewPair{1, 2}
	p2 := ViewPair{2, 3}
	p3 := ViewPair{3, 4}
	p4 := ViewPair{4, 5}
	p5 := ViewPair{2, 6}

	blocks := generateBlocks(root.QC, p1, p2, p3, p4, p5)
	for _, block := range blocks.blockList {
		err := f.AddBlock(block)
		require.NoError(t, err)
	}

	preferedQC := makeQC(6, blocks.blockMap[6].BlockID)
	err := f.AddQC(preferedQC)
	require.NoError(t, err)

	choiceQC, choiceBlock, err := f.MakeForkChoice(curView)
	require.NoError(t, err)
	require.Equal(t, preferedQC, choiceQC)
	require.Equal(t, blocks.blockMap[preferedQC.View], choiceBlock)
	notifier.AssertCalled(t, "OnForkChoiceGenerated", curView, preferedQC)
}

// NEWEST OVER LONGEST: NOT ENOUGH VOTES TO BUILD ON TOP OF NEWEST FORK: fork from newest
// As the leader of 7: we receive [1,2], [2,3], [3,4], [4,5], [2,6], but we don't receive enough votes to build a qc for block 6.
// the fork choice should return block and qc for 4
func TestNewestOverLongestForkFromNewest(t *testing.T) {
	curView := uint64(7)

	f, notifier, root := initNewestForkChoice(t, 1) // includes genesis block (v1)

	p1 := ViewPair{1, 2}
	p2 := ViewPair{2, 3}
	p3 := ViewPair{3, 4}
	p4 := ViewPair{4, 5}
	p5 := ViewPair{2, 6}

	blocks := generateBlocks(root.QC, p1, p2, p3, p4, p5)
	for _, block := range blocks.blockList {
		err := f.AddBlock(block)
		require.NoError(t, err)
	}

	// the fork choice should return block and qc for view 4
	preferedQC := makeQC(4, blocks.blockMap[4].BlockID)
	notifier.On("OnForkChoiceGenerated", curView, preferedQC).Return().Once()
	choiceQC, choiceBlock, err := f.MakeForkChoice(curView)
	require.NoError(t, err)
	require.Equal(t, preferedQC, choiceQC)
	require.Equal(t, blocks.blockMap[preferedQC.View], choiceBlock)
	notifier.AssertCalled(t, "OnForkChoiceGenerated", curView, preferedQC)
}

// FORK BELOW LOCKED: NOT ENOUGH VOTES TO BUILD ON TOP OF NEWEST FORK: fork from newest
// As the leader of 8: we receive [1,2], [2,3], [3,4], [4,5], [2,6], [6,7], but we don't receive enough votes to build a qc for block 7.
// the fork choice should return block and qc for 6
func TestForkBelowLockedForkFromNewest(t *testing.T) {
	curView := uint64(8)

	f, notifier, root := initNewestForkChoice(t, 1) // includes genesis block (v1)

	p1 := ViewPair{1, 2}
	p2 := ViewPair{2, 3}
	p3 := ViewPair{3, 4}
	p4 := ViewPair{4, 5}
	p5 := ViewPair{2, 6}
	p6 := ViewPair{6, 7}

	blocks := generateBlocks(root.QC, p1, p2, p3, p4, p5, p6)
	for _, block := range blocks.blockList {
		err := f.AddBlock(block)
		require.NoError(t, err)
	}

	// the fork choice should return block and qc for view 6
	preferedQC := makeQC(6, blocks.blockMap[6].BlockID)
	choiceQC, choiceBlock, err := f.MakeForkChoice(curView)
	require.NoError(t, err)
	require.Equal(t, preferedQC, choiceQC)
	require.Equal(t, blocks.blockMap[choiceQC.View], choiceBlock)
	notifier.AssertCalled(t, "OnForkChoiceGenerated", curView, preferedQC)
}

// FORK BELOW LOCKED: HAPPY PATH: extend newest
// As the leader of 8: we receive [1,2], [2,3], [3,4], [4,5], [2,6], [6,7], and receive enough votes to build QC for block 7.
// the fork choice should return block and qc for 7
func TestForkBelowLockedHappyPath(t *testing.T) {
	curView := uint64(8)

	f, notifier, root := initNewestForkChoice(t, 1) // includes genesis block (v1)

	p1 := ViewPair{1, 2}
	p2 := ViewPair{2, 3}
	p3 := ViewPair{3, 4}
	p4 := ViewPair{4, 5}
	p5 := ViewPair{2, 6}
	p6 := ViewPair{6, 7}

	blocks := generateBlocks(root.QC, p1, p2, p3, p4, p5, p6)
	for _, block := range blocks.blockList {
		err := f.AddBlock(block)
		require.NoError(t, err)
	}

	preferedQC := makeQC(7, blocks.blockMap[7].BlockID)
	err := f.AddQC(preferedQC)
	require.NoError(t, err)

	// the fork choice should return block and qc for view 7
	choiceQC, choiceBlock, err := f.MakeForkChoice(curView)
	require.NoError(t, err)
	require.Equal(t, preferedQC, choiceQC)
	require.Equal(t, blocks.blockMap[choiceQC.View], choiceBlock)
	notifier.AssertCalled(t, "OnForkChoiceGenerated", curView, preferedQC)
}

// Verifies notification callbacks
func TestOnQcIncorporated(t *testing.T) {
	notifier := &mocks.Consumer{}
	notifier.On("OnBlockIncorporated", mock.Anything).Return(nil)

	finalizationCallback := &mockfinalizer.Finalizer{}
	finalizationCallback.On("MakeFinal", mock.Anything).Return(nil)
	finalizationCallback.On("MakePending", mock.Anything, mock.Anything).Return(nil)

	// construct Finalizer
	root := makeRootBlock(t, 1)
	fnlzr, _ := finalizer.New(root, finalizationCallback, notifier)

	// construct ForkChoice, it will trigger OnQcIncorporated
	notifier.On("OnQcIncorporated", root.QC).Return(nil)
	fc, _ := NewNewestForkChoice(fnlzr, notifier)
	assert.NotNil(t, fc)

	f := forks.New(fnlzr, fc)

	p1 := ViewPair{1, 2}
	p2 := ViewPair{2, 3}

	blocks := generateBlocks(root.QC, p1, p2)

	for _, block := range blocks.blockList {
		// each call to AddBlock will trigger 'OnQcIncorporated'
		notifier.On("OnQcIncorporated", block.QC).Return(nil)
		err := f.AddBlock(block)
		require.NoError(t, err)
	}

	preferedQC := makeQC(3, blocks.blockMap[3].BlockID)
	// call to AddQC will trigger 'OnQcIncorporated'
	notifier.On("OnQcIncorporated", preferedQC).Return(nil)
	err := f.AddQC(preferedQC)
	require.NoError(t, err)
	notifier.AssertExpectations(t)
}

func generateBlocks(rootQC *model.QuorumCertificate, viewPairs ...ViewPair) *Blocks {
	blocks := &Blocks{
		blockMap: make(map[uint64]*model.Block),
	}
	qcs := &QCs{
		qcMap: make(map[uint64]*model.QuorumCertificate),
	}
	var lastBlockView uint64
	for _, viewPair := range viewPairs {
		var qc *model.QuorumCertificate
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

func initNewestForkChoice(t *testing.T, view uint64) (hs.Forks, *mocks.Consumer, *forks.BlockQC) {
	notifier := &mocks.Consumer{}
	notifier.On("OnBlockIncorporated", mock.Anything).Return(nil)
	notifier.On("OnFinalizedBlock", mock.Anything).Return(nil)
	notifier.On("OnForkChoiceGenerated", mock.Anything, mock.Anything).Return().Once()

	finalizationCallback := &mockfinalizer.Finalizer{}
	finalizationCallback.On("MakeFinal", mock.Anything).Return(nil)
	finalizationCallback.On("MakePending", mock.Anything, mock.Anything).Return(nil)

	// construct Finalizer
	root := makeRootBlock(t, view)
	fnlzr, _ := finalizer.New(root, finalizationCallback, notifier)

	// construct ForkChoice
	notifier.On("OnQcIncorporated", mock.Anything).Return(nil)
	fc, _ := NewNewestForkChoice(fnlzr, notifier)

	f := forks.New(fnlzr, fc)

	return f, notifier, root
}

func makeQC(view uint64, blockID flow.Identifier) *model.QuorumCertificate {
	return &model.QuorumCertificate{
		View:    view,
		BlockID: blockID,
	}
}

func (b *Blocks) AddBlock(block *model.Block) {
	b.blockMap[block.View] = block
	b.blockList = append(b.blockList, block)
}

func (q *QCs) AddQC(qc *model.QuorumCertificate) {
	q.qcMap[qc.View] = qc
	q.qcList = append(q.qcList, qc)
}
