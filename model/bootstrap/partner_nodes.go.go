package bootstrap

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

type PartnerNodeInfoPriv struct {
	Role           flow.Role
	Address        string
	NodeID         flow.Identifier
	NetworkPrivKey EncodableNetworkPrivKey
	StakingPrivKey EncodableStakingPrivKey
}

type PartnerNodeInfoPub struct {
	Role          flow.Role
	Address       string
	NodeID        flow.Identifier
	NetworkPubKey EncodableNetworkPubKey
	StakingPubKey EncodableStakingPubKey
}
