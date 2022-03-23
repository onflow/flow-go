package attacknetwork

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/utils/unittest"
)

// messageFixture creates and returns a randomly generated gRPC message that is sent between a corruptible conduit and the attack network.
// It also generates and returns the corresponding application-layer event of that message, which is sent between the attack network and the
// orchestrator.
func messageFixture(t *testing.T, codec network.Codec, protocol insecure.Protocol) (*insecure.Message, *insecure.Event, *flow.Identity) {
	// fixture for content of message
	originId := unittest.IdentifierFixture()

	var targetIds flow.IdentifierList
	targetNum := uint32(0)

	if protocol == insecure.Protocol_UNICAST {
		targetIds = unittest.IdentifierListFixture(1)
	} else {
		targetIds = unittest.IdentifierListFixture(10)
	}

	if protocol == insecure.Protocol_MULTICAST {
		targetNum = uint32(3)
	}

	channel := network.Channel("test-channel")
	content := &message.TestMessage{
		Text: fmt.Sprintf("this is a test message: %d", rand.Int()),
	}

	// encodes event to create payload
	payload, err := codec.Encode(content)
	require.NoError(t, err)

	// creates message that goes over gRPC.
	m := &insecure.Message{
		ChannelID: "test-channel",
		OriginID:  originId[:],
		TargetNum: targetNum,
		TargetIDs: flow.IdsToBytes(targetIds),
		Payload:   payload,
		Protocol:  protocol,
	}

	// creates corresponding event of that message that
	// is sent by attack network to orchestrator.
	e := &insecure.Event{
		CorruptedId:       originId,
		Channel:           channel,
		FlowProtocolEvent: content,
		Protocol:          protocol,
		TargetNum:         targetNum,
		TargetIds:         targetIds,
	}

	return m, e, unittest.IdentityFixture(unittest.WithNodeID(originId))
}

// messageFixtures creates and returns randomly generated gRCP messages and their corresponding protocol-level events.
// The messages are sent between a corruptible conduit and the attack network.
// The events are the corresponding protocol-level representation of messages.
func messageFixtures(t *testing.T, codec network.Codec, protocol insecure.Protocol, count int) ([]*insecure.Message, []*insecure.Event,
	flow.IdentityList) {
	msgs := make([]*insecure.Message, count)
	events := make([]*insecure.Event, count)
	identities := flow.IdentityList{}

	for i := 0; i < count; i++ {
		m, e, id := messageFixture(t, codec, protocol)

		msgs[i] = m
		events[i] = e
		// created identity must be unique
		require.NotContains(t, identities, id)
		identities = append(identities, id)
	}

	return msgs, events, identities
}
