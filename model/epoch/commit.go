package epoch

import (
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
)

type Commit struct {
	Counter         uint64
	ClusterQCs      []*model.QuorumCertificate
	DKGGroupKey     crypto.PublicKey
	DKGParticipants map[flow.Identifier]Participant
}

type Participant struct {
	Index    uint
	KeyShare crypto.PublicKey
}
