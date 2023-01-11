package bootstrap

import (
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
)

// PartnerNodeInfoPub represents public information about a partner/external
// node. It is identical to NodeInfoPub, but without weight information, as this
// is determined externally to the process that generates this information.
type PartnerNodeInfoPub struct {
	Role          flow.Role
	Address       string
	NodeID        flow.Identifier
	NetworkPubKey encodable.NetworkPubKey
	StakingPubKey encodable.StakingPubKey
}
