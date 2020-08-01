package epoch

import (
	"encoding/binary"

	hotstuff "github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
)

// TODO docs
type Commit struct {
	Counter         uint64
	ClusterQCs      []*hotstuff.QuorumCertificate
	DKGGroupKey     crypto.PublicKey
	DKGParticipants map[flow.Identifier]DKGParticipant
}

type DKGParticipant struct {
	Index    uint
	KeyShare crypto.PublicKey
}

// ID returns a unique ID for the epoch, based on the counter. This
// is used as a work-around for the current caching layer, which only
// suports flow entities keyed by ID for now.
func (c *Commit) ID() flow.Identifier {
	var commitID flow.Identifier
	binary.LittleEndian.PutUint64(commitID[:], c.Counter)
	return commitID
}
