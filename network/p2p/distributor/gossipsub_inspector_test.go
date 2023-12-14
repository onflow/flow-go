package distributor_test

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/distributor"
	p2pmsg "github.com/onflow/flow-go/network/p2p/message"
	mockp2p "github.com/onflow/flow-go/network/p2p/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestGossipSubInspectorNotification tests the GossipSub inspector notification by adding two consumers to the
// notification distributor component and sending a random set of notifications to the notification component. The test
// verifies that the consumers receive the notifications.
func TestGossipSubInspectorNotification(t *testing.T) {
	g := distributor.DefaultGossipSubInspectorNotificationDistributor(unittest.Logger())

	c1 := mockp2p.NewGossipSubInvalidControlMessageNotificationConsumer(t)
	c2 := mockp2p.NewGossipSubInvalidControlMessageNotificationConsumer(t)

	g.AddConsumer(c1)
	g.AddConsumer(c2)

	tt := invalidControlMessageNotificationListFixture(t, 100)

	c1Done := sync.WaitGroup{}
	c1Done.Add(len(tt))
	c1Seen := unittest.NewProtectedMap[peer.ID, struct{}]()
	c1.On("OnInvalidControlMessageNotification", mock.Anything).Run(func(args mock.Arguments) {
		notification, ok := args.Get(0).(*p2p.InvCtrlMsgNotif)
		require.True(t, ok)

		require.Contains(t, tt, notification)

		// ensure consumer see each peer once
		require.False(t, c1Seen.Has(notification.PeerID))
		c1Seen.Add(notification.PeerID, struct{}{})

		c1Done.Done()
	}).Return()

	c2Done := sync.WaitGroup{}
	c2Done.Add(len(tt))
	c2Seen := unittest.NewProtectedMap[peer.ID, struct{}]()
	c2.On("OnInvalidControlMessageNotification", mock.Anything).Run(func(args mock.Arguments) {
		notification, ok := args.Get(0).(*p2p.InvCtrlMsgNotif)
		require.True(t, ok)

		require.Contains(t, tt, notification)
		// ensure consumer see each peer once
		require.False(t, c2Seen.Has(notification.PeerID))
		c2Seen.Add(notification.PeerID, struct{}{})

		c2Done.Done()
	}).Return()

	cancelCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx, _ := irrecoverable.WithSignaler(cancelCtx)
	g.Start(ctx)

	unittest.RequireCloseBefore(t, g.Ready(), 100*time.Millisecond, "could not start distributor")

	for i := 0; i < len(tt); i++ {
		go func(i int) {
			require.NoError(t, g.Distribute(tt[i]))
		}(i)
	}

	unittest.RequireReturnsBefore(t, c1Done.Wait, 1*time.Second, "events are not received by consumer 1")
	unittest.RequireReturnsBefore(t, c2Done.Wait, 1*time.Second, "events are not received by consumer 2")
	cancel()
	unittest.RequireCloseBefore(t, g.Done(), 100*time.Millisecond, "could not stop distributor")
}

func invalidControlMessageNotificationListFixture(t *testing.T, n int) []*p2p.InvCtrlMsgNotif {
	list := make([]*p2p.InvCtrlMsgNotif, n)
	for i := 0; i < n; i++ {
		list[i] = invalidControlMessageNotificationFixture(t)
	}
	return list
}

// invalidControlMessageNotificationFixture creates an invalid control message notification fixture with a random message type.
func invalidControlMessageNotificationFixture(t *testing.T) *p2p.InvCtrlMsgNotif {
	msgType := []p2pmsg.ControlMessageType{p2pmsg.CtrlMsgGraft, p2pmsg.CtrlMsgPrune, p2pmsg.CtrlMsgIHave, p2pmsg.CtrlMsgIWant}[rand.Intn(4)]
	return &p2p.InvCtrlMsgNotif{
		PeerID:  unittest.PeerIdFixture(t),
		MsgType: msgType,
		Errors:  p2p.InvCtrlMsgErrs{p2p.NewInvCtrlMsgErr(fmt.Errorf("invalid control message"), p2p.CtrlMsgTopicTypeClusterPrefixed)},
	}
}
