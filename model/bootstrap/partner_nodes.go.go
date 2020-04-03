package bootstrap

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// PartnerNodeInfoPub represents public information about a partner/external
// node. It is identical to NodeInfoPub, but without stake information, as this
// is determined externally to the process that generates this information.
type PartnerNodeInfoPub struct {
	Role          flow.Role
	Address       string
	NodeID        flow.Identifier
	NetworkPubKey EncodableNetworkPubKey
	StakingPubKey EncodableStakingPubKey
}
