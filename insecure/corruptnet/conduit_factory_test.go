package corruptnet

import (
	"context"
	"testing"

	"github.com/onflow/flow-go/network/channels"

	"github.com/stretchr/testify/require"

	mockinsecure "github.com/onflow/flow-go/insecure/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestNewConduit_HappyPath checks when factory has an adapter registered and an egress controller,
// it can successfully create conduits.
func TestNewConduit_HappyPath(t *testing.T) {
	ccf := NewCorruptConduitFactory(unittest.Logger(), flow.BftTestnet)
	channel := channels.TestNetworkChannel
	require.NoError(t, ccf.RegisterEgressController(&mockinsecure.EgressController{}))
	require.NoError(t, ccf.RegisterAdapter(&mocknetwork.Adapter{}))

	c, err := ccf.NewConduit(context.Background(), channel)
	require.NoError(t, err)
	require.NotNil(t, c)
}

// TestRegisterAdapter_FailDoubleRegistration checks that CorruptibleConduitFactory can be registered with only one adapter.
func TestRegisterAdapter_FailDoubleRegistration(t *testing.T) {
	ccf := NewCorruptConduitFactory(unittest.Logger(), flow.BftTestnet)
	adapter := mocknetwork.NewAdapter(t)

	// registering adapter should be successful
	require.NoError(t, ccf.RegisterAdapter(adapter))

	// second attempt at registering adapter should fail
	require.ErrorContains(t, ccf.RegisterAdapter(adapter), "network adapter, one already exists")
}

// TestRegisterEgressController_FailDoubleRegistration checks that CorruptibleConduitFactory can be registered with only one egress controller.
func TestRegisterEgressController_FailDoubleRegistration(t *testing.T) {
	ccf := NewCorruptConduitFactory(unittest.Logger(), flow.BftTestnet)
	egressController := &mockinsecure.EgressController{}

	// registering egress controller should be successful
	require.NoError(t, ccf.RegisterEgressController(egressController))

	// second attempt at registering egress controller should fail
	require.ErrorContains(t, ccf.RegisterEgressController(egressController), "egress controller, one already exists")

}

// TestNewConduit_MissingAdapter checks when factory does not have an adapter registered (but does have egress controller),
// any attempts on creating a conduit fails with an error.
func TestNewConduit_MissingAdapter(t *testing.T) {
	ccf := NewCorruptConduitFactory(unittest.Logger(), flow.BftTestnet)
	channel := channels.TestNetworkChannel
	require.NoError(t, ccf.RegisterEgressController(&mockinsecure.EgressController{}))

	c, err := ccf.NewConduit(context.Background(), channel)
	require.ErrorContains(t, err, "missing a registered network adapter")
	require.Nil(t, c)
}

// TestNewConduit_MissingEgressController checks that test fails when factory doesn't have egress controller,
// but does have adapter.
func TestNewConduit_MissingEgressController(t *testing.T) {
	ccf := NewCorruptConduitFactory(unittest.Logger(), flow.BftTestnet)
	channel := channels.TestNetworkChannel
	require.NoError(t, ccf.RegisterAdapter(&mocknetwork.Adapter{}))

	c, err := ccf.NewConduit(context.Background(), channel)
	require.ErrorContains(t, err, "missing a registered egress controller")
	require.Nil(t, c)
}
