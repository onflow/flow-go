package rpc_inspector

import (
	"context"
	"testing"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/insecure/corruptlibp2p"
	"github.com/onflow/flow-go/insecure/internal"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/inspector"
	mockp2p "github.com/onflow/flow-go/network/p2p/mock"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestMetricsInspector_ObserveRPC ensures that the gossipsub rpc metrics inspector observes metrics for control messages as expected.
func TestMetricsInspector_ObserveRPC(t *testing.T) {
	t.Parallel()
	role := flow.RoleConsensus
	sporkID := unittest.IdentifierFixture()
	idProvider := unittest.NewUpdatableIDProvider(flow.IdentityList{})
	spammer := corruptlibp2p.NewGossipSubRouterSpammer(t, sporkID, role, idProvider)
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	messageCount := 100
	controlMessageCount := 5

	metricsObservedCount := atomic.NewInt64(0)
	mockMetricsObserver := mockp2p.NewGossipSubControlMetricsObserver(t)
	mockMetricsObserver.On("ObserveRPC", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			peerID, ok := args.Get(0).(peer.ID)
			require.True(t, ok)
			require.Equal(t, spammer.SpammerNode.ID(), peerID)
			rpc, ok := args.Get(1).(*pubsub.RPC)
			require.True(t, ok)
			// there are some default rpc messages exchanged between the nodes on startup
			// we can ignore those rpc messages not configured directly by this test
			if len(rpc.GetControl().GetPrune()) != 100 {
				return
			}
			require.True(t, messageCount == len(rpc.GetControl().GetPrune()))
			require.True(t, messageCount == len(rpc.GetControl().GetGraft()))
			require.True(t, messageCount == len(rpc.GetControl().GetIhave()))
			metricsObservedCount.Inc()
		})
	metricsInspector := inspector.NewControlMsgMetricsInspector(unittest.Logger(), mockMetricsObserver, 2)
	corruptInspectorFunc := corruptlibp2p.CorruptInspectorFunc(metricsInspector)
	victimNode, victimIdentity := p2ptest.NodeFixture(
		t,
		sporkID,
		t.Name(),
		idProvider,
		p2ptest.WithRole(role),
		internal.WithCorruptGossipSub(corruptlibp2p.CorruptGossipSubFactory(),
			corruptlibp2p.CorruptGossipSubConfigFactoryWithInspector(corruptInspectorFunc)),
	)
	idProvider.SetIdentities(flow.IdentityList{&victimIdentity, &spammer.SpammerId})
	metricsInspector.Start(signalerCtx)
	nodes := []p2p.LibP2PNode{victimNode, spammer.SpammerNode}
	startNodesAndEnsureConnected(t, signalerCtx, nodes, sporkID)
	spammer.Start(t)
	defer stopComponents(t, cancel, nodes, metricsInspector)
	// prepare to spam - generate control messages
	ctlMsgs := spammer.GenerateCtlMessages(controlMessageCount,
		corruptlibp2p.WithGraft(messageCount, channels.PushBlocks.String()),
		corruptlibp2p.WithPrune(messageCount, channels.PushBlocks.String()),
		corruptlibp2p.WithIHave(messageCount, 1000, channels.PushBlocks.String()))

	// start spamming the victim peer
	spammer.SpamControlMessage(t, victimNode, ctlMsgs)

	// eventually we should process each spammed control message and observe metrics for them
	require.Eventually(t, func() bool {
		return metricsObservedCount.Load() == int64(controlMessageCount)
	}, 5*time.Second, 10*time.Millisecond, "did not observe metrics for all control messages on time")
}
