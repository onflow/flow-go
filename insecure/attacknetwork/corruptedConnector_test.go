package attacknetwork

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestConnectorHappyPath(t *testing.T) {
	withMockCorruptibleConduitFactory(t, func(corruptedId flow.Identity, factory *mockCorruptibleConduitFactory) {
		connector := NewCorruptedConnector(flow.IdentityList{&corruptedId})

		go func() {
			msg := <-factory.attackerRegMsg

		}()

		connection, err := connector.Connect(context.Background(), corruptedId.NodeID)
		require.NoError(t, err)

		connection.SendMessage()

	})
}

func withMockCorruptibleConduitFactory(t *testing.T, run func(flow.Identity, *mockCorruptibleConduitFactory)) {
	corruptedIdentity := unittest.IdentityFixture(unittest.WithAddress("localhost:0"))

	// life-cycle management of attackNetwork.
	ctx, cancel := context.WithCancel(context.Background())
	ccfCtx, errChan := irrecoverable.WithSignaler(ctx)
	go func() {
		select {
		case err := <-errChan:
			t.Error("attack network startup encountered fatal error", err)
		case <-ctx.Done():
			return
		}
	}()

	ccf := newMockCorruptibleConduitFactory("localhost:5000")

	// starts corruptible conduit factory
	ccf.Start(ccfCtx)
	unittest.RequireCloseBefore(t, ccf.Ready(), 1*time.Second, "could not start corruptible conduit factory on time")

	run(*corruptedIdentity, ccf)

	// terminates attackNetwork
	cancel()
	unittest.RequireCloseBefore(t, ccf.Done(), 1*time.Second, "could not stop corruptible conduit on time")
}
