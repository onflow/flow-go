package topology

import (
	"context"
	"fmt"
	"log"

	fcrypto "github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/dht"

	discovery "github.com/libp2p/go-libp2p-discovery"
)

// DHTPeersTopology returns peers from inspecting the DHT
type DHTPeersTopology struct {
	fixedNodeID flow.Identity
	routing     *discovery.RoutingDiscovery
}

func NewDHTPeersTopology(nodeID flow.Identity, routing *discovery.RoutingDiscovery) DHTPeersTopology {
	return DHTPeersTopology{
		fixedNodeID: nodeID,
		routing:     routing,
	}
}

func (r DHTPeersTopology) GenerateFanout(ids flow.IdentityList, _ network.ChannelList) (flow.IdentityList, error) {

	peerInfo, err := p2p.PeerAddressInfo(r.fixedNodeID)
	// TODO: precompute this in a sync.Once
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	peers, err := discovery.FindPeers(ctx, r.routing, dht.FlowRendezVousStr)
	if err != nil {
		log.Fatal(err)
	}

	nodeIDs := flow.IdentityList{}
	for _, p := range peers {
		if p.ID == peerInfo.ID {
			continue
		}

		ip, port, err := p2p.IPPortFromMultiAddress(p.Addrs[0])
		if err != nil {
			// TODO: multierror
			continue
		}

		pk, err := p.ID.ExtractPublicKey()
		if err != nil {
			// TODO: multierror
			continue
		}
		// At this stage, pk is a lcrypto_pb.KeyType_ECDSA
		pkBytes, err := pk.Bytes()
		if err != nil {
			// TODO: multierror
			continue
		}

		flowPK, err := fcrypto.DecodePublicKey(fcrypto.ECDSAP256, pkBytes)
		if err != nil {
			// TODO: multierror
			continue
		}

		flowID, err := flow.PublicKeyToID(flowPK)
		if err != nil {
			// TODO: multierror
			continue
		}

		pID := flow.Identity{
			// we hash a Network key, so does libp2p see
			// https://github.com/libp2p/specs/pull/100/files
			NodeID:  flowID,
			Address: fmt.Sprintf("%v:%v", ip, port),
			Role:    flow.RoleAccess,
			Stake:   1000,
		}
		// when we learn of this peer is when is a good time to check in with the host h and
		// verify if h.Network().Connectedness(p.ID) != network.Connected
		// then we can dial the peer ... but that's the job of the PeerManager

		nodeIDs = append(nodeIDs, &pID)
	}

	return nodeIDs, nil
}
