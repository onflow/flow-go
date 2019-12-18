package pacemaker

import "github.com/dapperlabs/flow-go/engine/consensus/HotStuff/modules/def"

type Pacemaker interface {
	OnIncorporatedBlock(*def.Block)
	OnIncorporatedQuorumCertificate(*def.QuorumCertificate)

	Run()
}
