package defConAct

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/modules/crypto"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/modules/def"
)

type ViewChange struct {
	QC *def.QuorumCertificate

	ContentSignature *crypto.Signature
}

func NewViewChange(qc *def.QuorumCertificate, sig *crypto.Signature) *ViewChange {
	return &ViewChange{
		QC:               qc,
		ContentSignature: sig,
	}
}
