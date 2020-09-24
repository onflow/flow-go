package stub

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
)

// Hub is a test helper that mocks a network overlay.
// It maintains a set of network instances and enables them to directly exchange message
// over the memory.
type Hub struct {
	networks map[flow.Identifier]*Network
	Buffer   *Buffer
}

// NewNetworkHub creates and returns a new Hub instance.
func NewNetworkHub() *Hub {
	return &Hub{
		networks: make(map[flow.Identifier]*Network),
		Buffer:   NewBuffer(),
	}
}

// DeliverAll delivers all the buffered messages in the Network instances attached to the Hub
// to their destination.
// Note that the delivery of messages is done in asynchronous mode, i.e., sender and receiver are
// synchronized over delivery and not execution of the message.
func (h *Hub) DeliverAll() {
	for _, network := range h.networks {
		network.DeliverAll(false)
	}

}

// DeliverAllEventually attempts on delivery of all the buffered messages in the Network instances
// attached to this instance of Hub. Once the delivery is done, it evaluates and returns the
// condition function. It fails if delivery of all buffered messages in the Network instances
// attached to this Hub is not getting done within 10 seconds.
// Note that the delivery of messages is done in asynchronous mode, i.e., sender and receiver are
// synchronized over delivery and not execution of the message.
func (h *Hub) DeliverAllEventually(t *testing.T, condition func() bool) {
	h.DeliverAllEventuallyUntil(t, condition, time.Second*10, time.Millisecond*500)
}

// DeliverAllEventuallyUntil attempts attempts on delivery of all the buffered messages in the Network instances
// attached to this instance of Hub. Once the delivery is done, it evaluates and returns the
// condition function. It fails if delivery of all buffered messages in the Network instances
// attached to this Hub is not getting done within `waitFor` time interval.
// It checks the status of message deliveries at every `tick` time interval.
// Note that the delivery of messages is done in asynchronous mode, i.e., sender and receiver are
// synchronized over delivery and not execution of the message.
func (h *Hub) DeliverAllEventuallyUntil(t *testing.T, condition func() bool, waitFor time.Duration, tick time.Duration) {
	require.Eventually(t, func() bool {
		h.DeliverAll()
		return condition()
	}, waitFor, tick)
}

// GetNetwork returns the Network instance attached to the node ID.
func (h *Hub) GetNetwork(nodeID flow.Identifier) (*Network, bool) {
	net, ok := h.networks[nodeID]
	return net, ok
}

// AddNetwork stores the reference of the Network in the Hub, in order for networks to find
// other networks to send events directly.
func (h *Hub) AddNetwork(net *Network) {
	h.networks[net.GetID()] = net
}
