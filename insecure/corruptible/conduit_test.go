package corruptible

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	mockinsecure "github.com/onflow/flow-go/insecure/mock"
	"github.com/onflow/flow-go/insecure/proto"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestConduitRelayMessage_Publish evaluates that corruptible conduit relays all incoming publish events to its master.
func TestConduitRelayMessage_Publish(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	master := &mockinsecure.ConduitMaster{}
	channel := network.Channel("test-channel")

	c := &Conduit{
		ctx:     ctx,
		cancel:  cancel,
		channel: channel,
		master:  master,
	}

	event := unittest.MockEntityFixture()
	targetIds := unittest.IdentifierListFixture(10)

	params := []interface{}{ctx, event, channel, proto.Protocol_PUBLISH, uint32(0)}
	for _, id := range targetIds {
		params = append(params, id)
	}
	master.On("HandleIncomingEvent", params...).
		Return(nil).
		Once()

	err := c.Publish(event, targetIds...)
	require.NoError(t, err)
}

// TestConduitRelayMessage_Multicast evaluates that corruptible conduit relays all incoming multicast events to its master.
func TestConduitRelayMessage_Multicast(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	master := &mockinsecure.ConduitMaster{}
	channel := network.Channel("test-channel")
	num := 3 // targets of multicast

	c := &Conduit{
		ctx:     ctx,
		cancel:  cancel,
		channel: channel,
		master:  master,
	}

	event := unittest.MockEntityFixture()
	targetIds := unittest.IdentifierListFixture(10)

	params := []interface{}{ctx, event, channel, proto.Protocol_MULTICAST, uint32(num)}
	for _, id := range targetIds {
		params = append(params, id)
	}
	master.On("HandleIncomingEvent", params...).
		Return(nil).
		Once()

	err := c.Multicast(event, uint(num), targetIds...)
	require.NoError(t, err)
}

// TestConduitRelayMessage_Unicast evaluates that corruptible conduit relays all incoming unicast events to its master.
func TestConduitRelayMessage_Unicast(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	master := &mockinsecure.ConduitMaster{}
	channel := network.Channel("test-channel")

	c := &Conduit{
		ctx:     ctx,
		cancel:  cancel,
		channel: channel,
		master:  master,
	}

	event := unittest.MockEntityFixture()
	targetId := unittest.IdentifierFixture()

	master.On("HandleIncomingEvent", ctx, event, channel, proto.Protocol_UNICAST, uint32(0), targetId).
		Return(nil).
		Once()

	err := c.Unicast(event, targetId)
	require.NoError(t, err)
}

// TestConduitReflectError_Unicast evaluates that if master returns an error when the corruptible conduit sends relays a unicast to it,
// the error is reflected to the invoker of the corruptible conduit unicast.
func TestConduitReflectError_Unicast(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	master := &mockinsecure.ConduitMaster{}
	channel := network.Channel("test-channel")

	c := &Conduit{
		ctx:     ctx,
		cancel:  cancel,
		channel: channel,
		master:  master,
	}

	event := unittest.MockEntityFixture()
	targetId := unittest.IdentifierFixture()

	master.On("HandleIncomingEvent", ctx, event, channel, proto.Protocol_UNICAST, uint32(0), targetId).
		Return(fmt.Errorf("could not handle event")).
		Once()

	err := c.Unicast(event, targetId)
	require.Error(t, err)
}

// TestConduitReflectError_Multicast evaluates that if master returns an error when the corruptible conduit sends relays a multicast to it,
// the error is reflected to the invoker of the corruptible conduit multicast.
func TestConduitReflectError_Multicast(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	master := &mockinsecure.ConduitMaster{}
	channel := network.Channel("test-channel")
	num := 3 // targets of multicast

	c := &Conduit{
		ctx:     ctx,
		cancel:  cancel,
		channel: channel,
		master:  master,
	}

	event := unittest.MockEntityFixture()
	targetIds := unittest.IdentifierListFixture(10)

	params := []interface{}{ctx, event, channel, proto.Protocol_MULTICAST, uint32(num)}
	for _, id := range targetIds {
		params = append(params, id)
	}
	master.On("HandleIncomingEvent", params...).
		Return(fmt.Errorf("could not handle event")).
		Once()

	err := c.Multicast(event, uint(num), targetIds...)
	require.Error(t, err)
}

// TestConduitReflectError_Publish evaluates that if master returns an error when the corruptible conduit sends relays a multicast to it,
// the error is reflected to the invoker of the corruptible conduit multicast.
func TestConduitReflectError_Publish(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	master := &mockinsecure.ConduitMaster{}
	channel := network.Channel("test-channel")

	c := &Conduit{
		ctx:     ctx,
		cancel:  cancel,
		channel: channel,
		master:  master,
	}

	event := unittest.MockEntityFixture()
	targetIds := unittest.IdentifierListFixture(10)

	params := []interface{}{ctx, event, channel, proto.Protocol_PUBLISH, uint32(0)}
	for _, id := range targetIds {
		params = append(params, id)
	}
	master.On("HandleIncomingEvent", params...).
		Return(nil).
		Once()

	err := c.Publish(event, targetIds...)
	require.NoError(t, err)
}
