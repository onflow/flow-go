package pacemaker

import "github.com/dapperlabs/flow-go/engine/consensus/hotstuff/modules/def"

type Pacemaker interface {
	OnIncorporatedBlock(*def.Block)
	OnIncorporatedQuorumCertificate(*def.QuorumCertificate)

	Run()
}
