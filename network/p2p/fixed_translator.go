package p2p

import (
	"fmt"

	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/onflow/flow-go/model/flow"
)

// FixedTableIdentityTranslator implements an IDTranslator which translates ID's for a
// fixed list of identities.
type FixedTableIdentityTranslator struct {
	flow2p2p map[flow.Identifier]peer.ID
	p2p2flow map[peer.ID]flow.Identifier
}

func (t *FixedTableIdentityTranslator) GetFlowID(p peer.ID) (flow.Identifier, error) {
	nodeID, ok := t.p2p2flow[p]
	if !ok {
		return flow.ZeroID, fmt.Errorf("could not find a flow NodeID for peer %v", p)
	}
	return nodeID, nil
}

func (t *FixedTableIdentityTranslator) GetPeerID(n flow.Identifier) (peer.ID, error) {
	peerID, ok := t.flow2p2p[n]
	if !ok {
		return *new(peer.ID), fmt.Errorf("could not find a libp2p PeerID for flow NodeID %v", n)
	}
	return peerID, nil
}

func NewFixedTableIdentityTranslator(identities flow.IdentityList) (*FixedTableIdentityTranslator, error) {
	flow2p2p := make(map[flow.Identifier]peer.ID)
	p2p2flow := make(map[peer.ID]flow.Identifier)

	for _, identity := range identities {
		nodeID := identity.NodeID
		networkKey := identity.NetworkPubKey
		peerPK, err := LibP2PPublicKeyFromFlow(networkKey)
		if err != nil {
			return nil, err
		}

		peerID, err := peer.IDFromPublicKey(peerPK)
		if err != nil {
			return nil, err
		}

		flow2p2p[nodeID] = peerID
		p2p2flow[peerID] = nodeID
	}
	return &FixedTableIdentityTranslator{flow2p2p, p2p2flow}, nil
}
