package pacemaker

import (
	"github.com/dapperlabs/flow-go/engine/consensus/HotStuff/modules/def"
)

type Pacemaker interface {
	OnBlockIncorporated(*def.Block)
	OnQcFromVotesIncorporated(*def.QuorumCertificate)

	Run()
}

