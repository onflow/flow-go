package types

import (
	"bytes"
	"fmt"
)

// QCBlock contains a QC and a Block that the QC is pointing to
type QCBlock struct {
	qc    *QuorumCertificate
	block *BlockProposal
}

// NewQcBlock creates a QCBlock from a QC and the block its pointing to
func NewQcBlock(qc *QuorumCertificate, block *BlockProposal) (*QCBlock, error) {
	// sanity check
	if block.View() != qc.View || bytes.Equal(qc.BlockMRH, block.BlockMRH()) {
		return nil, fmt.Errorf("missmatching Block and qc")
	}
	return &QCBlock{qc: qc, block: block}, nil
}

func (qcb *QCBlock) View() uint64           { return qcb.qc.View }
func (qcb *QCBlock) QC() *QuorumCertificate { return qcb.qc }
func (qcb *QCBlock) Block() *BlockProposal  { return qcb.block }
