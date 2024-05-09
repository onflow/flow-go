package forks

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff/helper"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// BlockView specifies the data to create a block
type BlockView struct {
	// View is the view of the block to be created
	View uint64
	// BlockVersion is the version of the block for that view.
	// Useful for creating conflicting blocks at the same view.
	BlockVersion int
	// QCView is the view of the QC embedded in this block (also: the view of the block's parent)
	QCView uint64
	// QCVersion is the version of the QC for that view.
	QCVersion int
}

// QCIndex returns a unique identifier for the block's QC.
func (bv *BlockView) QCIndex() string {
	return fmt.Sprintf("%v-%v", bv.QCView, bv.QCVersion)
}

// BlockIndex returns a unique identifier for the block.
func (bv *BlockView) BlockIndex() string {
	return fmt.Sprintf("%v-%v", bv.View, bv.BlockVersion)
}

// BlockBuilder is a test utility for creating block structure fixtures.
type BlockBuilder struct {
	blockViews []*BlockView
}

func NewBlockBuilder() *BlockBuilder {
	return &BlockBuilder{
		blockViews: make([]*BlockView, 0),
	}
}

// Add adds a block with the given qcView and blockView. Returns self-reference for chaining.
func (bb *BlockBuilder) Add(qcView uint64, blockView uint64) *BlockBuilder {
	bb.blockViews = append(bb.blockViews, &BlockView{
		View:   blockView,
		QCView: qcView,
	})
	return bb
}

// GenesisBlock returns the genesis block, which is always finalized.
func (bb *BlockBuilder) GenesisBlock() *model.CertifiedBlock {
	return makeGenesis()
}

// AddVersioned adds a block with the given qcView and blockView.
// In addition, the version identifier of the QC embedded within the block
// is specified by `qcVersion`. The version identifier for the block itself
// (primarily for emulating different payloads) is specified by `blockVersion`.
// [(◄3) 4] denotes a block of view 4, with a qc for view 3
// [(◄3) 4'] denotes a block of view 4 that is different than [(◄3) 4], with a qc for view 3
// [(◄3) 4'] can be created by AddVersioned(3, 4, 0, 1)
// [(◄3') 4] can be created by AddVersioned(3, 4, 1, 0)
// Returns self-reference for chaining.
func (bb *BlockBuilder) AddVersioned(qcView uint64, blockView uint64, qcVersion int, blockVersion int) *BlockBuilder {
	bb.blockViews = append(bb.blockViews, &BlockView{
		View:         blockView,
		QCView:       qcView,
		BlockVersion: blockVersion,
		QCVersion:    qcVersion,
	})
	return bb
}

// Proposals returns a list of all proposals added to the BlockBuilder.
// Returns an error if the blocks do not form a connected tree rooted at genesis.
func (bb *BlockBuilder) Proposals() ([]*model.Proposal, error) {
	blocks := make([]*model.Proposal, 0, len(bb.blockViews))

	genesisBlock := makeGenesis()
	genesisBV := &BlockView{
		View:   genesisBlock.Block.View,
		QCView: genesisBlock.CertifyingQC.View,
	}

	qcs := make(map[string]*flow.QuorumCertificate)
	qcs[genesisBV.QCIndex()] = genesisBlock.CertifyingQC

	for _, bv := range bb.blockViews {
		qc, ok := qcs[bv.QCIndex()]
		if !ok {
			return nil, fmt.Errorf("test fail: no qc found for qc index: %v", bv.QCIndex())
		}
		payloadHash := makePayloadHash(bv.View, qc, bv.BlockVersion)
		var lastViewTC *flow.TimeoutCertificate
		if qc.View+1 != bv.View {
			lastViewTC = helper.MakeTC(helper.WithTCView(bv.View - 1))
		}
		proposal := &model.Proposal{
			Block: &model.Block{
				View:        bv.View,
				QC:          qc,
				PayloadHash: payloadHash,
			},
			LastViewTC: lastViewTC,
			SigData:    nil,
		}
		proposal.Block.BlockID = makeBlockID(proposal.Block)

		blocks = append(blocks, proposal)

		// generate QC for the new proposal
		qcs[bv.BlockIndex()] = &flow.QuorumCertificate{
			View:          proposal.Block.View,
			BlockID:       proposal.Block.BlockID,
			SignerIndices: nil,
			SigData:       nil,
		}
	}

	return blocks, nil
}

// Blocks returns a list of all blocks added to the BlockBuilder.
// Returns an error if the blocks do not form a connected tree rooted at genesis.
func (bb *BlockBuilder) Blocks() ([]*model.Block, error) {
	proposals, err := bb.Proposals()
	if err != nil {
		return nil, fmt.Errorf("BlockBuilder failed to generate proposals: %w", err)
	}
	return toBlocks(proposals), nil
}

func makePayloadHash(view uint64, qc *flow.QuorumCertificate, blockVersion int) flow.Identifier {
	return flow.MakeID(struct {
		View         uint64
		QC           *flow.QuorumCertificate
		BlockVersion uint64
	}{
		View:         view,
		QC:           qc,
		BlockVersion: uint64(blockVersion),
	})
}

func makeBlockID(block *model.Block) flow.Identifier {
	return flow.MakeID(struct {
		View        uint64
		QC          *flow.QuorumCertificate
		PayloadHash flow.Identifier
	}{
		View:        block.View,
		QC:          block.QC,
		PayloadHash: block.PayloadHash,
	})
}

// constructs the genesis block (identical for all calls)
func makeGenesis() *model.CertifiedBlock {
	genesis := &model.Block{
		View: 1,
	}
	genesis.BlockID = makeBlockID(genesis)

	genesisQC := &flow.QuorumCertificate{
		View:    1,
		BlockID: genesis.BlockID,
	}
	certifiedGenesisBlock, err := model.NewCertifiedBlock(genesis, genesisQC)
	if err != nil {
		panic(fmt.Sprintf("combining genesis block and genensis QC to certified block failed: %s", err.Error()))
	}
	return &certifiedGenesisBlock
}

// toBlocks converts the given proposals to slice of blocks
func toBlocks(proposals []*model.Proposal) []*model.Block {
	blocks := make([]*model.Block, 0, len(proposals))
	for _, b := range proposals {
		blocks = append(blocks, b.Block)
	}
	return blocks
}
