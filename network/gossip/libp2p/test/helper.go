package test

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/module/mock"
	"github.com/dapperlabs/flow-go/network/codec/json"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p/middleware"
)

// helper offers a set of functions that are shared among different tests
// CreateIDs creates and initializes count-many flow identifiers instances
func CreateIDs(count int) []*flow.Identity {
	identities := make([]*flow.Identity, 0)
	for i := 0; i < count; i++ {
		// defining id of node
		var nodeID [32]byte
		nodeID[0] = byte(i + 1)
		identity := &flow.Identity{
			NodeID: nodeID,
			Role:   flow.RoleCollection,
		}
		identities = append(identities, identity)
	}
	return identities
}

// createNetworks receives a slice of middlewares their associated flow identifiers,
// and for each middleware creates a network instance on top
// it returns the slice of created middlewares
// csize is the receive cache size of the nodes
func CreateNetworks(log zerolog.Logger, mws []*libp2p.Middleware, ids flow.IdentityList, csize int, dryrun bool, tops ...middleware.Topology) ([]*libp2p.Network, error) {
	count := len(mws)
	nets := make([]*libp2p.Network, 0)
	// create an empty identity list of size len(ids) to make sure the network fanout is set appropriately even before the nodes are started
	// identities are set to appropriate IP Port after the network and middleware are started
	identities := make(flow.IdentityList, len(ids))

	// if no topology is passed in, use the default topology for all networks
	if tops == nil {
		tops = make([]middleware.Topology, count)
		rpt := libp2p.NewRandPermTopology(flow.RoleCollection)
		for i := range tops {
			tops[i] = rpt
		}
	}

	for i := 0; i < count; i++ {
		// creates and mocks me
		me := &mock.Local{}
		me.On("NodeID").Return(ids[i].NodeID)
		me.On("NotMeFilter").Return(flow.IdentityFilter(filter.Any))
		net, err := libp2p.NewNetwork(log, json.NewCodec(), identities, me, mws[i], csize, tops[i])
		if err != nil {
			return nil, fmt.Errorf("could not create error %w", err)
		}

		nets = append(nets, net)
	}

	// if dryrun then don't actually start the network
	if !dryrun {
		for _, net := range nets {
			<-net.Ready()
		}
	}

	// set the identities to appropriate ip and port
	for i := range ids {
		// retrieves IP and port of the middleware
		var ip, port string
		var key crypto.PublicKey
		if !dryrun {
			m := mws[i]
			ip, port = m.GetIPPort()
			key = m.PublicKey()
		}

		// mocks an identity for the middleware
		id := flow.Identity{
			NodeID:        ids[i].NodeID,
			Address:       fmt.Sprintf("%s:%s", ip, port),
			Role:          flow.RoleCollection,
			Stake:         0,
			NetworkPubKey: key,
		}
		identities[i] = &id
	}

	// now that the network has started, address within the identity will have the actual port number
	// update the network with the new id
	for _, net := range nets {
		net.SetIDs(identities)
	}

	return nets, nil
}

// CreateMiddleware receives an ids slice and creates and initializes a middleware instances for each id
func CreateMiddleware(log zerolog.Logger, identities []*flow.Identity) ([]*libp2p.Middleware, error) {
	metrics := metrics.NewNoopCollector()
	count := len(identities)
	mws := make([]*libp2p.Middleware, 0)
	for i := 0; i < count; i++ {

		key, err := GenerateNetworkingKey(identities[i].NodeID)
		if err != nil {
			return nil, err
		}

		// creating middleware of nodes
		mw, err := libp2p.NewMiddleware(log, json.NewCodec(), "0.0.0.0:0", identities[i].NodeID, key, metrics, libp2p.DefaultMaxPubSubMsgSize)
		if err != nil {
			return nil, err
		}

		mws = append(mws, mw)
	}
	return mws, nil
}

type SnapshotMock struct {
	ids flow.IdentityList
}

func (s *SnapshotMock) Identities(filters ...flow.IdentityFilter) (flow.IdentityList, error) {
	return s.ids, nil
}

func (s *SnapshotMock) Identity(nodeID flow.Identifier) (*flow.Identity, error) {
	return nil, fmt.Errorf(" not implemented")
}

func (s *SnapshotMock) Clusters() (*flow.ClusterList, error) {
	return nil, fmt.Errorf(" not implemented")
}

func (s *SnapshotMock) Head() (*flow.Header, error) {
	return nil, fmt.Errorf(" not implemented")
}

func (s *SnapshotMock) Seal() (flow.Seal, error) {
	return flow.Seal{}, fmt.Errorf(" not implemented")
}

// GenerateNetworkingKey generates a Flow ECDSA key using the given seed
func GenerateNetworkingKey(s flow.Identifier) (crypto.PrivateKey, error) {
	seed := make([]byte, crypto.KeyGenSeedMinLenECDSASecp256k1)
	copy(seed, s[:])
	return crypto.GeneratePrivateKey(crypto.ECDSASecp256k1, seed)
}
