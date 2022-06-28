package test

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff/forks"
	"github.com/onflow/flow-go/consensus/hotstuff/helper"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// BlockView specifies the data to create a block
type BlockView struct {
	View uint64 // the view of the block to be created

	// the version of the block for that view.  useful for creating a different block of the same view with a different version
	BlockVersion int

	QCView uint64 // the view for the QC to be built on top of

	// the version of the QC for that view.
	QCVersion int
}

func (bv *BlockView) QCIndex() string {
	return fmt.Sprintf("%v-%v", bv.QCView, bv.QCVersion)
}

func (bv *BlockView) BlockIndex() string {
	return fmt.Sprintf("%v-%v", bv.View, bv.BlockVersion)
}

type BlockBuilder struct {
	blockViews []*BlockView
}

func NewBlockBuilder() *BlockBuilder {
	return &BlockBuilder{
		blockViews: make([]*BlockView, 0),
	}
}

func (f *BlockBuilder) Add(qcView uint64, blockView uint64) {
	f.blockViews = append(f.blockViews, &BlockView{
		View:   blockView,
		QCView: qcView,
	})
}

// [3,4] denotes a block of view 4, with a qc of view 3.
// [3,4'] denotes a block of view 4, with a qc of view 3, but has a different BlockID than [3,4],
// [3,4'] can be created by AddVersioned(3, 4, 0, 1)
// [3',4] can be created by AddVersioned(3, 4, 1, 0)
func (f *BlockBuilder) AddVersioned(qcView uint64, blockView uint64, qcversion int, blockversion int) {
	f.blockViews = append(f.blockViews, &BlockView{
		View:         blockView,
		QCView:       qcView,
		BlockVersion: blockversion,
		QCVersion:    qcversion,
	})
}

func (f *BlockBuilder) Blocks() ([]*model.Proposal, error) {
	blocks := make([]*model.Proposal, 0, len(f.blockViews))

	genesisBQ := makeGenesis()
	genesisBV := &BlockView{
		View:   genesisBQ.Block.View,
		QCView: genesisBQ.QC.View,
	}

	qcs := make(map[string]*flow.QuorumCertificate)
	qcs[genesisBV.QCIndex()] = genesisBQ.QC

	for _, bv := range f.blockViews {
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

func makeGenesis() *forks.BlockQC {
	genesis := &model.Block{
		View: 1,
	}
	genesis.BlockID = makeBlockID(genesis)

	genesisQC := &flow.QuorumCertificate{
		View:    1,
		BlockID: genesis.BlockID,
	}
	genesisBQ := &forks.BlockQC{
		Block: genesis,
		QC:    genesisQC,
	}
	return genesisBQ
}
