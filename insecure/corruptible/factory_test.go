package corruptible_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/insecure/corruptible"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/codec/cbor"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestRegisterAdapter(t *testing.T) {
	f := corruptible.NewCorruptibleConduitFactory(unittest.IdentifierFixture(), cbor.NewCodec())

	adapter := &mocknetwork.Adapter{}

	// registering adapter must go successful
	require.NoError(t, f.RegisterAdapter(adapter))

	// second attempt on registering adapter must fail
	require.Error(t, f.RegisterAdapter(adapter))
}

// TestNewConduit_HappyPath checks when factory has an adapter registered, it can successfully
// create conduits.
func TestNewConduit_HappyPath(t *testing.T) {
	f := corruptible.NewCorruptibleConduitFactory(unittest.IdentifierFixture(), cbor.NewCodec())
	channel := network.Channel("test-channel")

	adapter := &mocknetwork.Adapter{}

	require.NoError(t, f.RegisterAdapter(adapter))

	c, err := f.NewConduit(context.Background(), channel)
	require.NoError(t, err)
	require.NotNil(t, c)
}
