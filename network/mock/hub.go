package mock

import (
	"github.com/pkg/errors"
)

// Hub is a value that stores mocked networks in order for them to send events directly
type Hub struct {
	networks map[string]*Network
	buffer   *Buffer
}

// NewNetworkHub returns a MockHub value with empty network slice
func NewNetworkHub() *Hub {
	return &Hub{
		networks: make(map[string]*Network),
		buffer:   NewBuffer(),
	}
}

// GetNetwork returns the Network by the network ID (or node ID)
func (hub *Hub) GetNetwork(networkID string) *Network {
	network, ok := hub.networks[networkID]
	if !ok {
		return nil
	}
	return network
}

// Plug stores the reference of the network in the hub object, in order for networks to find
// other network to send events directly
func (hub *Hub) Plug(net *Network) {
	hub.networks[net.GetID()] = net
}

func (hub *Hub) Buffer(from string, engineID uint8, event interface{}, targetIDs []string) {
	hub.buffer.Save(from, engineID, event, targetIDs)
}

func (hub *Hub) sendToAllTargets(m *Message) error {
	key := eventKey(m.EngineID, m.Event)
	for _, nodeID := range m.TargetIDs {
		// Find the network of the targeted node
		receiverNetwork := hub.GetNetwork(nodeID)
		if receiverNetwork == nil {
			return errors.Errorf("Network can not find a node by ID %v", nodeID)
		}

		// Check if the given engine already received the event.
		// This prevents a node receiving the same event twice.
		if receiverNetwork.haveSeen(key) {
			continue
		}

		// mark the peer has seen the event
		receiverNetwork.seen(key)

		// Find the engine of the targeted network
		receiverEngine, ok := receiverNetwork.engines[m.EngineID]
		if !ok {
			return errors.Errorf("Network can not find engine ID: %v for node: %v", m.EngineID, nodeID)
		}

		// Find the engine of the targeted network
		// Call `Process` to let receiver engine receive the event directly.
		err := receiverEngine.Process(m.From, m.Event)

		if err != nil {
			return errors.Wrapf(err, "senderEngine failed to process event: %v", m.Event)
		}
	}
	return nil
}

func (hub *Hub) FlushAll() {
	hub.buffer.Flush(hub.sendToAllTargets)
}

func (hub *Hub) BlockByMessageTypeAndFlushAll(shouldBlock func(*Message) bool) {
	hub.buffer.Flush(func(m *Message) error {
		if shouldBlock(m) {
			return nil
		}
		return hub.sendToAllTargets(m)
	})
}
