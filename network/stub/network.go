package stub

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/network/p2p/conduit"
)

// Network is a mocked Network layer made for testing engine's behavior.
// It represents the Network layer of a single node. A node can attach several engines of
// itself to the Network, and hence enabling them send and receive message.
// When an engine is attached on a Network instance, the mocked Network delivers
// all engine's events to others using an in-memory delivery mechanism.
type Network struct {
	mocknetwork.Network
	ctx context.Context
	sync.Mutex
	myId           flow.Identifier                               // used to represent information of the attached node.
	hub            *Hub                                          // used to attach Network layers of nodes together.
	engines        map[channels.Channel]network.MessageProcessor // used to keep track of attached engines of the node.
	seenEventIDs   map[string]struct{}                           // used to keep track of event IDs seen by attached engines.
	qCD            chan struct{}                                 // used to stop continuous delivery mode of the Network.
	conduitFactory network.ConduitFactory
}

func WithConduitFactory(factory network.ConduitFactory) func(*Network) {
	return func(n *Network) {
		n.conduitFactory = factory
	}
}

var _ network.EngineRegistry = (*Network)(nil)
var _ network.ConduitAdapter = (*Network)(nil)

// NewNetwork create a mocked Network.
// The committee has the identity of the node already, so only `committee` is needed
// in order for a mock hub to find each other.
func NewNetwork(t testing.TB, myId flow.Identifier, hub *Hub, opts ...func(*Network)) *Network {
	net := &Network{
		ctx:            context.Background(),
		myId:           myId,
		hub:            hub,
		engines:        make(map[channels.Channel]network.MessageProcessor),
		seenEventIDs:   make(map[string]struct{}),
		qCD:            make(chan struct{}),
		conduitFactory: conduit.NewDefaultConduitFactory(),
	}

	for _, opt := range opts {
		opt(net)
	}

	// mocks the Start, Ready, and Done behavior of the network.
	net.On("Start", mock.Anything).Return()
	ready := make(chan struct{})
	close(ready)
	net.On("Ready", mock.Anything).Return(func() <-chan struct{} {
		return ready
	})

	done := make(chan struct{})
	close(done)
	net.On("Done", mock.Anything).Return(func() <-chan struct{} {
		return done
	})

	require.NoError(t, net.conduitFactory.RegisterAdapter(net))

	// AddNetwork the Network to a hub so that Networks can find each other.
	hub.AddNetwork(net)
	return net
}

// GetID returns the identity of the attached node.
func (n *Network) GetID() flow.Identifier {
	return n.myId
}

// Register registers an Engine of the attached node to the channel via a Conduit, and returns the
// Conduit instance.
func (n *Network) Register(channel channels.Channel, engine network.MessageProcessor) (network.Conduit, error) {
	n.Lock()
	defer n.Unlock()
	_, ok := n.engines[channel]
	if ok {
		return nil, errors.Errorf("channel already taken (%s)", channel)
	}

	c, err := n.conduitFactory.NewConduit(n.ctx, channel)
	if err != nil {
		return nil, fmt.Errorf("could not create a conduit on the channel: %w", err)
	}

	n.engines[channel] = engine

	return c, nil
}

func (n *Network) UnRegisterChannel(channel channels.Channel) error {
	n.Lock()
	defer n.Unlock()
	delete(n.engines, channel)
	return nil
}

// submit is called when the attached Engine to the channel is sending an event to an
// Engine attached to the same channel on another node or nodes.
func (n *Network) submit(channel channels.Channel, event interface{}, targetIDs ...flow.Identifier) error {
	m := &PendingMessage{
		From:      n.GetID(),
		Channel:   channel,
		Event:     event,
		TargetIDs: targetIDs,
	}

	n.buffer(m)

	return nil
}

// unicast is called when the attached Engine to the channel is sending an event to a single target
// Engine attached to the same channel on another node.
func (n *Network) UnicastOnChannel(channel channels.Channel, event interface{}, targetID flow.Identifier) error {
	m := &PendingMessage{
		From:      n.GetID(),
		Channel:   channel,
		Event:     event,
		TargetIDs: []flow.Identifier{targetID},
	}

	n.buffer(m)
	return nil
}

// publish is called when the attached Engine is sending an event to a group of Engines attached to the
// same channel on other nodes based on selector.
// In this test helper implementation, publish uses submit method under the hood.
func (n *Network) PublishOnChannel(channel channels.Channel, event interface{}, targetIDs ...flow.Identifier) error {

	if len(targetIDs) == 0 {
		return fmt.Errorf("publish found empty target ID list for the message")
	}

	return n.submit(channel, event, targetIDs...)
}

// multicast is called when an engine attached to the channel is sending an event to a number of randomly chosen
// Engines attached to the same channel on other nodes. The targeted nodes are selected based on the selector.
// In this test helper implementation, multicast uses submit method under the hood.
func (n *Network) MulticastOnChannel(channel channels.Channel, event interface{}, num uint, targetIDs ...flow.Identifier) error {
	var err error
	targetIDs, err = flow.Sample(num, targetIDs...)
	if err != nil {
		return fmt.Errorf("sampling failed: %w", err)
	}
	return n.submit(channel, event, targetIDs...)
}

// buffer saves the message into the pending buffer of the Network hub.
// Buffering process of a message imitates its transmission over an unreliable Network.
// In specific, it emulates the process of dispatching the message out of the sender.
func (n *Network) buffer(msg *PendingMessage) {
	n.hub.Buffer.Save(msg)
}

// DeliverAll sends all pending messages to the receivers. The receivers
// might be triggered to forward messages to its peers, so this function will
// block until all receivers have done their forwarding, and there is no more message
// in the Network to deliver.
func (n *Network) DeliverAll(syncOnProcess bool) {
	n.hub.Buffer.DeliverRecursive(func(m *PendingMessage) {
		_ = n.sendToAllTargets(m, syncOnProcess)
	})
}

// DeliverAllExcept flushes all pending messages in the buffer except
// those that satisfy the shouldDrop predicate function. All messages that
// satisfy the shouldDrop predicate are permanently dropped.
// The message receivers might be triggered to forward some messages to their peers,
// so this function will block until all receivers have done their forwarding,
// and there is no more message in the Network to deliver.
//
// If syncOnProcess is true, the sender and receiver are synchronized on processing the message.
// Otherwise they sync on delivery of the message.
func (n *Network) DeliverAllExcept(syncOnProcess bool, shouldDrop func(*PendingMessage) bool) {
	n.hub.Buffer.DeliverRecursive(func(m *PendingMessage) {
		if shouldDrop(m) {
			return
		}
		_ = n.sendToAllTargets(m, syncOnProcess)
	})
}

// DeliverSome delivers all messages in the buffer that satisfy the
// shouldDeliver predicate. Any messages that are not delivered remain in the
// buffer.
//
// If syncOnProcess is true, the sender and receiver are synchronized on processing the message.
// Otherwise they sync on delivery of the message.
func (n *Network) DeliverSome(syncOnProcess bool, shouldDeliver func(*PendingMessage) bool) {
	n.hub.Buffer.Deliver(func(m *PendingMessage) bool {
		if shouldDeliver(m) {
			return n.sendToAllTargets(m, syncOnProcess) != nil
		}
		return false
	})
}

// sendToAllTargets send a message to all its targeted nodes if the targeted
// node has not yet seen it.
// sync parameter defines whether the sender and receiver are synced over processing or delivery of
// message.
// If syncOnProcess is set true, sender and receiver are synced over processing of the message, i.e., the method call
// gets blocking till the message is processed at destination.
// If syncOnProcess is set false, sender and receiver are synced over delivery of the message, i.e., the method call
// returns once the message is delivered at destination (and not necessarily processed).
func (n *Network) sendToAllTargets(m *PendingMessage, syncOnProcess bool) error {
	key, err := eventKey(m.From, m.Channel, m.Event)
	if err != nil {
		return fmt.Errorf("could not generate event key for event: %w", err)
	}

	for _, nodeID := range m.TargetIDs {
		// finds the Network of the targeted node
		receiverNetwork, exist := n.hub.GetNetwork(nodeID)
		if !exist {
			continue
		}

		// finds the engine of the targeted Network
		err := receiverNetwork.processWithEngine(syncOnProcess, key, m)
		if err != nil {
			return fmt.Errorf("could not process message for nodeID: %v, %w", nodeID, err)
		}
	}
	return nil
}

func (n *Network) processWithEngine(syncOnProcess bool, key string, m *PendingMessage) error {
	n.Lock()
	defer n.Unlock()

	// checks if the given engine already received the event.
	// this prevents a node receiving the same event twice.
	if _, ok := n.seenEventIDs[key]; ok {
		return nil
	}
	n.seenEventIDs[key] = struct{}{}

	receiverEngine, ok := n.engines[m.Channel]
	if !ok {
		return fmt.Errorf("could find engine ID: %v", m.Channel)
	}

	if syncOnProcess {
		// sender and receiver are synced over processing the message
		if err := receiverEngine.Process(m.Channel, m.From, m.Event); err != nil {
			return fmt.Errorf("receiver engine failed to process event (%v): %w", m.Event, err)
		}
	} else {
		// sender and receiver are synced over delivery of message
		go func() {
			_ = receiverEngine.Process(m.Channel, m.From, m.Event)
		}()
	}
	return nil
}

// StartConDev starts the continuous delivery mode of the Network.
// In this mode, the Network continuously checks the nodes' buffer
// every `updateInterval` milliseconds, and delivers all the pending
// messages. `recursive` determines whether the delivery is in recursive mode or not
func (n *Network) StartConDev(updateInterval time.Duration, recursive bool) {
	timer := time.NewTicker(updateInterval)

	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		wg.Done()
		for {
			select {
			case <-timer.C:
				n.DeliverAll(recursive)
			case <-n.qCD:
				// stops continuous delivery mode
				return
			}
		}
	}()

	// waits till the internal goroutine starts
	wg.Wait()
}

// StopConDev stops the continuous deliver mode of the Network.
func (n *Network) StopConDev() {
	close(n.qCD)
}

func (n *Network) ReportMisbehaviorOnChannel(_ channels.Channel, _ network.MisbehaviorReport) {
	// no-op for stub network.
}
