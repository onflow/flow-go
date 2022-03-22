package attacknetwork_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/insecure/attacknetwork"
	"github.com/onflow/flow-go/insecure/corruptible"
	mockinsecure "github.com/onflow/flow-go/insecure/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network/codec/cbor"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestConnectorHappyPath(t *testing.T) {
	withCorruptibleConduitFactory(t, func(identity flow.Identity, factory *corruptible.ConduitFactory) {
		corruptedIds := flow.IdentityList{&identity}
		withAttackNetwork(t, corruptedIds, func(attackNetwork *attacknetwork.AttackNetwork) {

		})
	})
}

func withAttackNetwork(t *testing.T, corruptedIds flow.IdentityList, run func(*attacknetwork.AttackNetwork)) {
	codec := cbor.NewCodec()
	o := &mockinsecure.AttackOrchestrator{}
	connector := attacknetwork.NewCorruptedConnector(corruptedIds)
	attackNetwork, err := attacknetwork.NewAttackNetwork(unittest.Logger(), "localhost:0", codec, o, connector, corruptedIds)
	require.NoError(t, err)

	// life-cycle management of attackNetwork.
	ctx, cancel := context.WithCancel(context.Background())
	attackNetworkCtx, errChan := irrecoverable.WithSignaler(ctx)
	go func() {
		select {
		case err := <-errChan:
			t.Error("attackNetwork startup encountered fatal error", err)
		case <-ctx.Done():
			return
		}
	}()

	// starts corruptible conduit factory
	attackNetwork.Start(attackNetworkCtx)
	unittest.RequireCloseBefore(t, attackNetwork.Ready(), 1*time.Second, "could not start attack network on time")

	run(attackNetwork)

	// terminates attackNetwork
	cancel()
	unittest.RequireCloseBefore(t, attackNetwork.Done(), 1*time.Second, "could not stop attack network on time")
}

func withCorruptibleConduitFactory(t *testing.T, run func(flow.Identity, *corruptible.ConduitFactory)) {
	codec := cbor.NewCodec()
	corruptedIdentity := unittest.IdentityFixture()

	// life-cycle management of attackNetwork.
	ctx, cancel := context.WithCancel(context.Background())
	ccfCtx, errChan := irrecoverable.WithSignaler(ctx)
	go func() {
		select {
		case err := <-errChan:
			t.Error("attackNetwork startup encountered fatal error", err)
		case <-ctx.Done():
			return
		}
	}()
	ccf := corruptible.NewCorruptibleConduitFactory(unittest.Logger(), flow.BftTestnet, corruptedIdentity.NodeID, codec, "localhost:0")

	// starts corruptible conduit factory
	ccf.Start(ccfCtx)
	unittest.RequireCloseBefore(t, ccf.Ready(), 1*time.Second, "could not start corruptible conduit factory on time")

	run(*corruptedIdentity, ccf)

	// terminates attackNetwork
	cancel()
	unittest.RequireCloseBefore(t, ccf.Done(), 1*time.Second, "could not stop corruptible conduit on time")
}
