package network

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine"
	ghostclient "github.com/onflow/flow-go/engine/ghost/client"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/integration/tests/common"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestNetwork tests the 1-k messaging at the network layer using the default Flow network topology
// No real nodes are created, instead only Ghost nodes are used to restrict testing to only the network module
func TestNetwork(t *testing.T) {

	// define what nodes and how many instances of each need to be created (role => count e.g. consensus = 3, creates 3 ghost consensus nodes)
	nodeCounts := map[flow.Role]int{flow.RoleAccess: 2, flow.RoleCollection: 1, flow.RoleConsensus: 3, flow.RoleExecution: 2, flow.RoleVerification: 1}

	var nodes []testnet.NodeConfig
	id := uint(1)
	// create node configs
	for role, nc := range nodeCounts {
		for i := 0; i < nc; i++ {
			// create a ghost node config for each node
			n := testnet.NewNodeConfig(role, testnet.WithLogLevel(zerolog.FatalLevel), testnet.WithIDInt(id), testnet.AsGhost())
			nodes = append(nodes, n)
			id++
		}
	}

	// collect all the real ids of the nodes
	var ids []flow.Identifier
	for _, n := range nodes {
		ids = append(ids, n.Identifier)
	}
	require.GreaterOrEqual(t, len(ids), 2)

	conf := testnet.NewNetworkConfig("network_test", nodes)

	net := testnet.PrepareFlowNetwork(t, conf)

	var wg sync.WaitGroup
	defer wg.Wait()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	net.Start(ctx)
	defer net.Remove()

	// first node sends a message to all the other nodes
	sender := ids[0]
	targets := ids[1:]

	event := &message.TestMessage{
		Text: fmt.Sprintf("hello"),
	}

	// kick off a read loop for each of the nodes (except the first)
	for _, id := range targets {
		wg.Add(1)
		go readLoop(ctx, id, net, &wg, t, sender, event.Text)
	}

	// get the sender container and relay an echo message via it to all the other nodes
	ghostContainer := net.ContainerByID(sender)
	ghostClient, err := common.GetGhostClient(ghostContainer)
	require.NoError(t, err)

	// seed a message, it should propagate to all nodes.
	// (unlike regular nodes, a ghost node subscribes to all topics)
	err = ghostClient.Send(ctx, engine.PushGuarantees, event, targets...)
	require.NoError(t, err)

	// wait for all read loops to finish
	unittest.AssertReturnsBefore(t, wg.Wait, 5*time.Second, "timed out waiting for nodes to receive message")
}

func readLoop(ctx context.Context, id flow.Identifier, net *testnet.FlowNetwork, wg *sync.WaitGroup,
	t *testing.T, expectedOrigin flow.Identifier, expectedMsg string) {
	defer wg.Done()

	// get the ghost container
	ghostContainer := net.ContainerByID(id)

	// get a ghost client connected to the ghost node
	ghostClient, err := common.GetGhostClient(ghostContainer)
	if !assert.NoError(t, err) {
		return
	}

	// subscribe to all the events the ghost execution node will receive
	var msgReader *ghostclient.FlowMessageStreamReader
	for attempts := 0; attempts < 10; attempts++ {
		msgReader, err = ghostClient.Subscribe(ctx)
		if err == nil {
			break
		}
	}

	if !(assert.NoError(t, err) && assert.NotNil(t, msgReader)) {
		return
	}

	for {

		select {
		case <-ctx.Done():
			return
		default:
		}

		actualOriginID, event, err := msgReader.Next()
		if !assert.NoError(t, err) {
			return
		}

		switch v := event.(type) {
		case *message.TestMessage:
			t.Logf("%s: %s: %s", id.String(), actualOriginID.String(), v.Text)
			assert.Equal(t, expectedOrigin, actualOriginID)
			assert.Equal(t, expectedMsg, v.Text)
			return
		default:
		}
	}
}
