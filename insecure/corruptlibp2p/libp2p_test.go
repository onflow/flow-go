package corruptlibp2p

import (
	"context"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network/p2p"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
	"github.com/onflow/flow-go/utils/unittest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGetGossipSubParams(t *testing.T) {
	gossipSubParams := getGossipSubParams()

	require.Equal(t, gossipSubParams, gossipSubParams)
}

func TestSpam(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	sporkId := unittest.IdentifierFixture()
	defer cancel()

	count := 5
	nodes := make([]p2p.LibP2PNode, 0, 5)
	ids := flow.IdentityList{}
	inbounds := make([]chan string, 0, 5)

	//disallowedList := map[*flow.Identity]struct{}{}

	for i := 0; i < count; i++ {
		handler, inbound := p2ptest.StreamHandlerFixture(t)
		node, id := p2ptest.NodeFixture(
			t,
			sporkId,
			t.Name(),
			p2ptest.WithRole(flow.RoleConsensus),
			p2ptest.WithDefaultStreamHandler(handler),
			// enable peer manager, with a 1-second refresh rate, and connection pruning enabled.
			//p2ptest.WithPeerManagerEnabled(true, 1*time.Second, func() peer.IDSlice {
			//	list := make(peer.IDSlice, 0)
			//	for _, id := range ids {
			//		if _, ok := disallowedList[id]; ok {
			//			continue
			//		}
			//
			//		pid, err := unittest.PeerIDFromFlowID(id)
			//		require.NoError(t, err)
			//
			//		list = append(list, pid)
			//	}
			//	return list
			//}),
			//p2ptest.WithConnectionGater(testutils.NewConnectionGater(func(pid peer.ID) error {
			//	for id := range disallowedList {
			//		bid, err := unittest.PeerIDFromFlowID(id)
			//		require.NoError(t, err)
			//		if bid == pid {
			//			return fmt.Errorf("disallow-listed")
			//		}
			//	}
			//
			//	return nil
			//})),
		)

		nodes = append(nodes, node)
		ids = append(ids, &id)
		inbounds = append(inbounds, inbound)
	}

	p2ptest.StartNodes(t, signalerCtx, nodes, 1*time.Second)
	defer p2ptest.StopNodes(t, nodes, cancel, 1*time.Second)
}
