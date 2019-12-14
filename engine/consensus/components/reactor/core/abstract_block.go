package core

import "github.com/dapperlabs/flow-go/engine/consensus/modules/def"

type AbstractBlock interface {
	// Hash returns the block's hash
	Hash() []byte

	// View returns the block's view number
	View() uint64

	// QC returns the block's embedded QC pointing to the block's parent
	QC() *def.QuorumCertificate

	// Block returns the block itself
	Block() *def.Block
}
