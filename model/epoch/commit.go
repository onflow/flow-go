package epoch

import (
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/model/dkg"
)

type Commit struct {
	Counter    uint64
	DKGData    *dkg.PublicData
	ClusterQCs []*model.QuorumCertificate
}
