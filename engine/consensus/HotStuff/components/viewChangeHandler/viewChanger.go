package viewChangeHandler

import "github.com/dapperlabs/flow-go/engine/consensus/HotStuff/modules/def"

// ViewChanger sends and receives `ViewChange`s
type ViewChangeHandler struct {
}

func (v *ViewChangeHandler) OnSendViewChange(qc *def.QuorumCertificate) {
	panic("Implement me")
}
