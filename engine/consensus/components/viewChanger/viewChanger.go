package voteAggregator

import "github.com/dapperlabs/flow-go/engine/consensus/modules/def"

// ViewChanger sends and receives `ViewChange`s
type ViewChanger struct {
}

func (v *ViewChanger) OnSendViewChange(qc *def.QuorumCertificate) {
	panic("Implement me")
}
