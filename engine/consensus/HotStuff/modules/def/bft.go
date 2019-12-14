package def

type BFT interface {
	// TODO: This definitely needs a better name
	PassMsgToBFT(interface{})
	Lock()
	Unlock()

	GetCurrentView() uint64
	GetGenericQC() *QuorumCertificate
	GetNextPrimary() NodeIdentity
	GetMainChainBlockFromHash([]byte) *Block
	EnsureBlock([]byte)
	// GetLatestBlock() *Block
	GetLatestFinalisedBlock() *Block
	GetNextFinalisedBlock() *Block

	SetGenericQC(*QuorumCertificate)
	IncrementCurrentView()
}
