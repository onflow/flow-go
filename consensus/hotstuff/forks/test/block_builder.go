package test

import (
	"fmt"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/forks"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/model/flow"
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

func (f *BlockBuilder) Blocks() ([]*model.Block, error) {
	blocks := make([]*model.Block, 0, len(f.blockViews))

	genesisBQ := makeGenesis()
	genesisBV := &BlockView{
		View:   genesisBQ.Block.View,
		QCView: genesisBQ.QC.View,
	}

	qcs := make(map[string]*model.QuorumCertificate)
	qcs[genesisBV.QCIndex()] = genesisBQ.QC

	for _, bv := range f.blockViews {
		qc, ok := qcs[bv.QCIndex()]
		if !ok {
			return nil, fmt.Errorf("test fail: no qc found for qc index: %v", bv.QCIndex())
		}
		payloadHash := makePayloadHash(bv.View, qc, bv.BlockVersion)
		block := &model.Block{
			View:        bv.View,
			QC:          qc,
			PayloadHash: payloadHash,
		}
		block.BlockID = makeBlockID(block)

		blocks = append(blocks, block)

		// generate QC for the new block
		qcs[bv.BlockIndex()] = &model.QuorumCertificate{
			View:      block.View,
			BlockID:   block.BlockID,
			SignerIDs: nil,
			SigData:   nil,
		}
	}

	return blocks, nil
}

func makePayloadHash(view uint64, qc *model.QuorumCertificate, blockVersion int) flow.Identifier {
	return flow.MakeID(struct {
		View         uint64
		QC           *model.QuorumCertificate
		BlockVersion int
	}{
		View:         view,
		QC:           qc,
		BlockVersion: blockVersion,
	})
}

func makeBlockID(block *model.Block) flow.Identifier {
	return flow.MakeID(struct {
		View        uint64
		QC          *model.QuorumCertificate
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

	genesisQC := &model.QuorumCertificate{
		View:    1,
		BlockID: genesis.BlockID,
	}
	genesisBQ := &forks.BlockQC{
		Block: genesis,
		QC:    genesisQC,
	}
	return genesisBQ
}
