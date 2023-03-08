package distributor_test

import (
	"context"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/distributor"
	mockp2p "github.com/onflow/flow-go/network/p2p/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestDisallowListNotificationDistributor tests the disallow list notification distributor by adding two consumers to the
// notification distributor component and sending a random set of notifications to the notification component. The test
// verifies that the consumers receive the notifications and that each consumer sees each notification only once.
func TestDisallowListNotificationDistributor(t *testing.T) {
	d := distributor.DefaultDisallowListNotificationDistributor(unittest.Logger())

	c1 := mockp2p.NewDisallowListNotificationConsumer(t)
	c2 := mockp2p.NewDisallowListNotificationConsumer(t)

	d.AddConsumer(c1)
	d.AddConsumer(c2)

	tt := disallowListUpdateNotificationsFixture(50)

	c1Done := sync.WaitGroup{}
	c1Done.Add(len(tt))
	c1Seen := unittest.NewProtectedMap[flow.Identifier, struct{}]()
	c1.On("OnDisallowListNotification", mock.Anything).Run(func(args mock.Arguments) {
		n, ok := args.Get(0).(*p2p.DisallowListUpdateNotification)
		require.True(t, ok)

		require.Contains(t, tt, n)

		// ensure consumer see each peer once
		hash := flow.MerkleRoot(n.DisallowList...)
		require.False(t, c1Seen.Has(hash))
		c1Seen.Add(hash, struct{}{})

		c1Done.Done()
	}).Return()

	c2Done := sync.WaitGroup{}
	c2Done.Add(len(tt))
	c2Seen := unittest.NewProtectedMap[flow.Identifier, struct{}]()
	c2.On("OnDisallowListNotification", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		n, ok := args.Get(0).(*p2p.DisallowListUpdateNotification)
		require.True(t, ok)

		require.Contains(t, tt, n)

		// ensure consumer see each peer once
		hash := flow.MerkleRoot(n.DisallowList...)
		require.False(t, c2Seen.Has(hash))
		c2Seen.Add(hash, struct{}{})

		c2Done.Done()
	}).Return()

	cancelCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx, _ := irrecoverable.WithSignaler(cancelCtx)
	d.Start(ctx)

	unittest.RequireCloseBefore(t, d.Ready(), 100*time.Millisecond, "could not start distributor")

	for i := 0; i < len(tt); i++ {
		go func(i int) {
			require.NoError(t, d.DistributeBlockListNotification(tt[i].DisallowList))
		}(i)
	}

	unittest.RequireReturnsBefore(t, c1Done.Wait, 1*time.Second, "events are not received by consumer 1")
	unittest.RequireReturnsBefore(t, c2Done.Wait, 1*time.Second, "events are not received by consumer 2")
	cancel()
	unittest.RequireCloseBefore(t, d.Done(), 100*time.Millisecond, "could not stop distributor")
}

func disallowListUpdateNotificationsFixture(n int) []*p2p.DisallowListUpdateNotification {
	tt := make([]*p2p.DisallowListUpdateNotification, n)
	for i := 0; i < n; i++ {
		tt[i] = disallowListUpdateNotificationFixture()
	}
	return tt
}

func disallowListUpdateNotificationFixture() *p2p.DisallowListUpdateNotification {
	return &p2p.DisallowListUpdateNotification{
		DisallowList: unittest.IdentifierListFixture(rand.Int()%100 + 1),
	}
}
