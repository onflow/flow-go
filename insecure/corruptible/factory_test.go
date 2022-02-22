package corruptible

import (
	"context"
	"testing"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/codec/cbor"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestRegisterAdapter(t *testing.T) {
	f := NewCorruptibleConduitFactory(unittest.IdentifierFixture(), cbor.NewCodec())

	adapter := &mocknetwork.Adapter{}

	// registering adapter must go successful
	require.NoError(t, f.RegisterAdapter(adapter))

	// second attempt on registering adapter must fail
	require.Error(t, f.RegisterAdapter(adapter))
}

// TestNewConduit_HappyPath checks when factory has an adapter registered, it can successfully
// create conduits.
func TestNewConduit_HappyPath(t *testing.T) {
	f := NewCorruptibleConduitFactory(unittest.IdentifierFixture(), cbor.NewCodec())
	channel := network.Channel("test-channel")

	adapter := &mocknetwork.Adapter{}

	require.NoError(t, f.RegisterAdapter(adapter))

	c, err := f.NewConduit(context.Background(), channel)
	require.NoError(t, err)
	require.NotNil(t, c)
}

// TestNewConduit_MissingAdapter checks when factory does not have an adapter registered,
// any attempts on creating a conduit fails with an error.
func TestNewConduit_MissingAdapter(t *testing.T) {
	f := NewCorruptibleConduitFactory(unittest.IdentifierFixture(), cbor.NewCodec())
	channel := network.Channel("test-channel")

	c, err := f.NewConduit(context.Background(), channel)
	require.Error(t, err)
	require.Nil(t, c)
}

// TestRegisterAttacker checks attacker registration to a corruptible conduit factory can be done
// only once.
func TestRegisterAttacker(t *testing.T) {
	f := NewCorruptibleConduitFactory(unittest.IdentifierFixture(), cbor.NewCodec())
	f.attacker = &mockAttacker{}

	event := unittest.MockEntityFixture()
	targetIds := unittest.IdentifierListFixture(10)
	channel := network.Channel("test-channel")

	err := f.HandleIncomingEvent(context.Background(), event, channel, insecure.Protocol_MULTICAST, uint32(3), targetIds...)
	require.NoError(t, err)
}

type mockAttacker struct {
	incomingBuffer chan *insecure.Message
}

func (m *mockAttacker) Observe(_ context.Context, in *insecure.Message, _ ...grpc.CallOption) (*empty.Empty, error) {
	m.incomingBuffer <- in
	return &empty.Empty{}, nil
}
