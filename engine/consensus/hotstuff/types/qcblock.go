package types

// QCBlock contains a QC and a Block that the QC is pointing to
type QCBlock struct {
	QC    *QuorumCertificate
	Block *BlockProposal
}

// NewQC creates a QCBlock from a QC and the block its pointing to
func NewQC(qc *QuorumCertificate, block *BlockProposal) *QCBlock {
	return &QCBlock{
		QC:    qc,
		Block: block,
	}
}
