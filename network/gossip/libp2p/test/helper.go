package test

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/codec/json"
	"github.com/onflow/flow-go/network/gossip/libp2p"
	"github.com/onflow/flow-go/network/gossip/libp2p/topology"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/utils/unittest"
)

var rootBlockID = unittest.IdentifierFixture().String()

// helper offers a set of functions that are shared among different tests
// CreateIDs creates and initializes count-many flow identifiers instancesd
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

// CreateNetworks receives a slice of middlewares their associated flow identifiers,
// and for each middleware creates a network instance on top
// it returns the slice of created middlewares
// csize is the receive cache size of the nodes
func CreateNetworks(t *testing.T,
	logger zerolog.Logger,
	ids flow.IdentityList,
	mws []*libp2p.Middleware,
	tops []*topology.Topology,
	csize int,
	dryrun bool) []*libp2p.Network {

	nets := make([]*libp2p.Network, 0)

	// create an empty identity list of size len(ids) to make sure the network fanout is set appropriately even before the nodes are started
	// identities are set to appropriate IP Port after the network and middleware are started
	identities := make(flow.IdentityList, 0)

	for i := 0; i < len(ids); i++ {
		// creates and mocks me
		me := &mock.Local{}
		me.On("NodeID").Return(ids[i].NodeID)
		me.On("NotMeFilter").Return(flow.IdentityFilter(filter.Any))
		net, err := libp2p.NewNetwork(logger,
			json.NewCodec(),
			identities,
			me,
			ids[i].Role,
			mws[i],
			csize,
			*tops[i],
			metrics.NewNoopCollector())

		require.NoError(t, err, fmt.Errorf("could not create network: %w", err))
		nets = append(nets, net)
	}

	// if dryrun then don't actually start the network
	if !dryrun {
		for _, net := range nets {
			<-net.Ready()
		}
	}

	identities = make(flow.IdentityList, len(ids))
	// set the identities to appropriate ip and port
	for i := range ids {
		// retrieves IP and port of the middleware
		var ip, port string
		var err error
		var key crypto.PublicKey
		if !dryrun {
			m := mws[i]
			ip, port, err = m.GetIPPort()
			require.NoError(t, err, fmt.Errorf("could not get ip and port: %w", err))
			key = m.PublicKey()
		}

		// mocks an identity for the middleware
		id := flow.Identity{
			NodeID:        ids[i].NodeID,
			Address:       fmt.Sprintf("%s:%s", ip, port),
			Role:          ids[i].Role,
			Stake:         0,
			NetworkPubKey: key,
		}
		identities[i] = &id
	}

	// now that the network has started, address within the identity will have the actual port number
	// update the network with the new ids
	for _, net := range nets {
		net.SetIDs(identities)
	}

	// update whitelist of each of the middleware after the network ids have been updated
	if !dryrun {
		for _, m := range mws {
			err := m.UpdateAllowList()
			require.NoError(t, err, fmt.Errorf("could not update allow list: %w", err))
		}
	}

	return nets
}

// CreateMiddleware receives an ids slice and creates and initializes a middleware instances for each id
func CreateMiddleware(t *testing.T, log zerolog.Logger, identities []*flow.Identity) []*libp2p.Middleware {
	metrics := metrics.NewNoopCollector()
	count := len(identities)
	mws := make([]*libp2p.Middleware, 0)
	for i := 0; i < count; i++ {

		key, err := GenerateNetworkingKey(identities[i].NodeID)
		require.NoError(t, err)

		// creating middleware of nodes
		mw, err := libp2p.NewMiddleware(log,
			json.NewCodec(),
			"0.0.0.0:0",
			identities[i].NodeID,
			key,
			metrics,
			libp2p.DefaultMaxUnicastMsgSize,
			libp2p.DefaultMaxPubSubMsgSize,
			rootBlockID)
		require.NoError(t, err)
		mws = append(mws, mw)
	}
	return mws
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

// CreateTopologies is a test helper on receiving an identity list, creates a topology per identity
// and returns the slice of topologies. It distinguishes between the topology creation of collection nodes
// and other nodes.
func CreateTopologies(t *testing.T, state protocol.State, identities flow.IdentityList) []*topology.Topology {
	tops := make([]*topology.Topology, 0)
	for _, id := range identities {
		var top topology.Topology
		var err error

		top, err = topology.NewTopicAwareTopology(id.NodeID, state)

		require.NoError(t, err)
		tops = append(tops, &top)
	}
	return tops
}

// OptionalSleep introduces a sleep to allow nodes to heartbeat and discover each other (only needed when using PubSub)
func optionalSleep(send ConduitSendWrapperFunc) {
	sendFuncName := runtime.FuncForPC(reflect.ValueOf(send).Pointer()).Name()
	if strings.Contains(sendFuncName, "Multicast") || strings.Contains(sendFuncName, "Publish") {
		time.Sleep(2 * time.Second)
	}
}
