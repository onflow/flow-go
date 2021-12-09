// Package p2p encapsulates the libp2p library
package p2p

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/libp2p/go-libp2p-core/host"
	libp2pnet "github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/libp2p/go-libp2p-core/routing"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/rs/zerolog"

	flownet "github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/p2p/unicast"
	validator "github.com/onflow/flow-go/network/validator/pubsub"
)

const (

	// PingTimeout is maximum time to wait for a ping reply from a remote node
	PingTimeout = time.Second * 4

	// maximum number of attempts to be made to connect to a remote node for 1-1 direct communication
	maxConnectAttempt = 3

	// timeout for FindPeer queries to the routing system
	// TODO: is this a sensible value?
	findPeerQueryTimeout = 10 * time.Second
)

// Node is a wrapper around the LibP2P host.
type Node struct {
	sync.Mutex
	unicastManager *unicast.Manager
	host           host.Host                              // reference to the libp2p host (https://godoc.org/github.com/libp2p/go-libp2p-core/host)
	pubSub         *pubsub.PubSub                         // reference to the libp2p PubSub component
	logger         zerolog.Logger                         // used to provide logging
	topics         map[flownet.Topic]*pubsub.Topic        // map of a topic string to an actual topic instance
	subs           map[flownet.Topic]*pubsub.Subscription // map of a topic string to an actual subscription
	routing        routing.Routing
	pCache         *protocolPeerCache
}

// Stop terminates the libp2p node.
func (n *Node) Stop() (chan struct{}, error) {
	var result error
	done := make(chan struct{})
	n.logger.Debug().Msg("unsubscribing from all topics")
	for t := range n.topics {
		if err := n.UnSubscribe(t); err != nil {
			result = multierror.Append(result, err)
		}
	}

	n.logger.Debug().Msg("stopping libp2p node")
	if err := n.host.Close(); err != nil {
		result = multierror.Append(result, err)
	}

	n.logger.Debug().Msg("closing peer store")
	// to prevent peerstore routine leak (https://github.com/libp2p/go-libp2p/issues/718)
	if err := n.host.Peerstore().Close(); err != nil {
		n.logger.Debug().Err(err).Msg("closing peer store")
		result = multierror.Append(result, err)
	}

	if result != nil {
		close(done)
		return done, result
	}

	go func(done chan struct{}) {
		defer close(done)
		addrs := len(n.host.Network().ListenAddresses())
		ticker := time.NewTicker(time.Millisecond * 2)
		defer ticker.Stop()
		timeout := time.After(time.Second)
		for addrs > 0 {
			// wait for all listen addresses to have been removed
			select {
			case <-timeout:
				n.logger.Error().Int("port", addrs).Msg("listen addresses still open")
				return
			case <-ticker.C:
				addrs = len(n.host.Network().ListenAddresses())
			}
		}

		n.logger.Debug().Msg("libp2p node stopped successfully")
	}(done)

	return done, nil
}

// AddPeer adds a peer to this node by adding it to this node's peerstore and connecting to it
func (n *Node) AddPeer(ctx context.Context, peerInfo peer.AddrInfo) error {
	return n.host.Connect(ctx, peerInfo)
}

// RemovePeer closes the connection with the peer.
func (n *Node) RemovePeer(peerID peer.ID) error {
	err := n.host.Network().ClosePeer(peerID)
	if err != nil {
		return fmt.Errorf("failed to remove peer %s: %w", peerID, err)
	}
	return nil
}

func (n *Node) GetPeersForProtocol(pid protocol.ID) peer.IDSlice {
	pMap := n.pCache.getPeers(pid)
	peers := make(peer.IDSlice, 0, len(pMap))
	for p := range pMap {
		peers = append(peers, p)
	}
	return peers
}

// CreateStream returns an existing stream connected to the peer if it exists, or creates a new stream with it.
func (n *Node) CreateStream(ctx context.Context, peerID peer.ID) (libp2pnet.Stream, error) {
	lg := n.logger.With().Str("peer_id", peerID.Pretty()).Logger()

	// If we do not currently have any addresses for the given peer, stream creation will almost
	// certainly fail. If this Node was configured with a routing system, we can try to use it to
	// look up the address of the peer.
	if len(n.host.Peerstore().Addrs(peerID)) == 0 && n.routing != nil {
		lg.Info().Msg("address not found in peer store, searching for peer in routing system")

		var err error
		func() {
			timedCtx, cancel := context.WithTimeout(ctx, findPeerQueryTimeout)
			defer cancel()
			// try to find the peer using the routing system
			_, err = n.routing.FindPeer(timedCtx, peerID)
		}()

		if err != nil {
			lg.Warn().Err(err).Msg("address not found in both peer store and routing system")
		} else {
			lg.Debug().Msg("address not found in peer store, but found in routing system search")
		}
	}
	stream, dialAddrs, err := n.unicastManager.CreateStream(ctx, peerID, maxConnectAttempt)
	if err != nil {
		return nil, flownet.NewPeerUnreachableError(fmt.Errorf("could not create stream (peer_id: %s, dialing address(s): %v): %w", peerID,
			dialAddrs, err))
	}

	lg.Debug().Str("dial_address", fmt.Sprintf("%v", dialAddrs)).Msg("stream successfully created to remote peer")
	return stream, nil
}

// GetIPPort returns the IP and Port the libp2p node is listening on.
func (n *Node) GetIPPort() (string, string, error) {
	return IPPortFromMultiAddress(n.host.Network().ListenAddresses()...)
}

// Subscribe subscribes the node to the given topic and returns the subscription
// Currently only one subscriber is allowed per topic.
// NOTE: A node will receive its own published messages.
func (n *Node) Subscribe(topic flownet.Topic, validators ...validator.MessageValidator) (*pubsub.Subscription, error) {
	n.Lock()
	defer n.Unlock()

	// Check if the topic has been already created and is in the cache
	n.pubSub.GetTopics()
	tp, found := n.topics[topic]
	var err error
	if !found {
		topicValidator := validator.TopicValidator(validators...)
		if err := n.pubSub.RegisterTopicValidator(
			topic.String(), topicValidator, pubsub.WithValidatorInline(true),
		); err != nil {
			n.logger.Err(err).Str("topic", topic.String()).Msg("failed to register topic validator, aborting subscription")
			return nil, fmt.Errorf("failed to register topic validator: %w", err)
		}

		tp, err = n.pubSub.Join(topic.String())
		if err != nil {
			if err := n.pubSub.UnregisterTopicValidator(topic.String()); err != nil {
				n.logger.Err(err).Str("topic", topic.String()).Msg("failed to unregister topic validator")
			}

			return nil, fmt.Errorf("could not join topic (%s): %w", topic, err)
		}

		n.topics[topic] = tp
	}

	// Create a new subscription
	s, err := tp.Subscribe()
	if err != nil {
		return s, fmt.Errorf("could not subscribe to topic (%s): %w", topic, err)
	}

	// Add the subscription to the cache
	n.subs[topic] = s

	n.logger.Debug().
		Str("topic", topic.String()).
		Msg("subscribed to topic")
	return s, err
}

// UnSubscribe cancels the subscriber and closes the topic.
func (n *Node) UnSubscribe(topic flownet.Topic) error {
	n.Lock()
	defer n.Unlock()
	// Remove the Subscriber from the cache
	if s, found := n.subs[topic]; found {
		s.Cancel()
		n.subs[topic] = nil
		delete(n.subs, topic)
	}

	tp, found := n.topics[topic]
	if !found {
		err := fmt.Errorf("could not find topic (%s)", topic)
		return err
	}

	if err := n.pubSub.UnregisterTopicValidator(topic.String()); err != nil {
		n.logger.Err(err).Str("topic", topic.String()).Msg("failed to unregister topic validator")
	}

	// attempt to close the topic
	err := tp.Close()
	if err != nil {
		err = fmt.Errorf("could not close topic (%s): %w", topic, err)
		return err
	}
	n.topics[topic] = nil
	delete(n.topics, topic)

	n.logger.Debug().
		Str("topic", topic.String()).
		Msg("unsubscribed from topic")
	return err
}

// Publish publishes the given payload on the topic
func (n *Node) Publish(ctx context.Context, topic flownet.Topic, data []byte) error {
	ps, found := n.topics[topic]
	if !found {
		return fmt.Errorf("could not find topic (%s)", topic)
	}
	err := ps.Publish(ctx, data)
	if err != nil {
		return fmt.Errorf("could not publish top topic (%s): %w", topic, err)
	}
	return nil
}

// Host returns pointer to host object of node.
func (n *Node) Host() host.Host {
	return n.host
}

func (n *Node) WithDefaultUnicastProtocol(defaultHandler libp2pnet.StreamHandler, preferred []unicast.ProtocolName) error {
	n.unicastManager.WithDefaultHandler(defaultHandler)
	for _, p := range preferred {
		err := n.unicastManager.Register(p)
		if err != nil {
			return fmt.Errorf("could not register unicast protocls")
		}
	}

	return nil
}

// IsConnected returns true is address is a direct peer of this node else false
func (n *Node) IsConnected(peerID peer.ID) (bool, error) {
	isConnected := n.host.Network().Connectedness(peerID) == libp2pnet.Connected
	return isConnected, nil
}
