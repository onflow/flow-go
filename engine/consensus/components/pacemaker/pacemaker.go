package pacemaker

import "github.com/dapperlabs/flow-go/engine/consensus/componentsdef"

type Pacemaker interface {
	OnIncorporatedBlock(*def.Block)
	OnIncorporatedQuorumCertificate(*def.QuorumCertificate)

	Run()
}
