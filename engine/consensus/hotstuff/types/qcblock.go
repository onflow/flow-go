package types

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
)

// QCBlock contains a QC and a Block that the QC is pointing to
type QCBlock struct {
	qc    *QuorumCertificate
	block *BlockProposal
}

// NewQcBlock creates a QCBlock from a QC and the block its pointing to
func NewQcBlock(qc *QuorumCertificate, block *BlockProposal) (*QCBlock, error) {
	// sanity check
	if block.View() != qc.View || qc.BlockID != block.BlockID() {
		return nil, fmt.Errorf("QC does not mathc block")
	}
	return &QCBlock{qc: qc, block: block}, nil
}

func (qcb *QCBlock) BlockID() flow.Identifier { return qcb.block.BlockID() }
func (qcb *QCBlock) View() uint64             { return qcb.block.View() }
func (qcb *QCBlock) Block() *BlockProposal    { return qcb.block }
func (qcb *QCBlock) QC() *QuorumCertificate   { return qcb.qc }
