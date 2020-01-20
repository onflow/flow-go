// Package libp2p encapsulates the libp2p library
package libp2p

import (
	"context"
	"fmt"
	"sync"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	"github.com/libp2p/go-libp2p-core/protocol"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-tcp-transport"
	"github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

// A unique Libp2p protocol ID for Flow (https://docs.libp2p.io/concepts/protocols/)
// All nodes communicate with each other using this protocol
const (
	FlowLibP2PProtocolID protocol.ID = "/flow/push/0.0.1"
)

// NodeAddress is used to define a libp2p node
type NodeAddress struct {
	// Name is the friendly node Name e.g. "node1" (not to be confused with the libp2p node id)
	Name string
	IP   string
	Port string
}

// P2PNode manages the the libp2p node.
type P2PNode struct {
	sync.Mutex
	name       string                             // friendly human readable Name of the node
	libP2PHost host.Host                          // reference to the libp2p host (https://godoc.org/github.com/libp2p/go-libp2p-core/host)
	logger     zerolog.Logger                     // for logging
	ps         *pubsub.PubSub                     // the reference to the pubsub instance
	topics     map[FlowTopic]*pubsub.Topic        // map of a topic string to an actual topic instance
	subs       map[FlowTopic]*pubsub.Subscription // map of a topic string to an actual subscription
}

// Start starts a libp2p node on the given address.
func (p *P2PNode) Start(ctx context.Context, n NodeAddress, logger zerolog.Logger, handler network.StreamHandler) error {
	p.Lock()
	defer p.Unlock()
	p.name = n.Name
	p.logger = logger
	addr := getLocationMultiaddrString(n)
	sourceMultiAddr, err := multiaddr.NewMultiaddr(addr)
	if err != nil {
		return err
	}

	key, err := GetPublicKey(n.Name)
	if err != nil {
		err = errors.Wrapf(err, "could not generate public key for %s", p.name)
		return err
	}

	// libp2p.New constructs a new libp2p Host.
	// Other options can be added here.
	host, err := libp2p.New(
		ctx,
		libp2p.ListenAddrs(sourceMultiAddr),
		//libp2p.NoSecurity,
		libp2p.Identity(key),
		libp2p.Transport(tcp.NewTCPTransport), // the default transport unnecessarily brings in a websocket listener
	)
	if err != nil {
		return errors.Wrapf(err, "could not construct libp2p host for %s", p.name)
	}
	p.libP2PHost = host

	host.SetStreamHandler(FlowLibP2PProtocolID, handler)

	// Creating a new PubSub instance of the type GossipSub
	p.ps, err = pubsub.NewGossipSub(ctx, p.libP2PHost)

	if err != nil {
		return errors.Wrapf(err, "unable to start pubsub %s", p.name)
	}

	p.topics = make(map[FlowTopic]*pubsub.Topic)
	p.subs = make(map[FlowTopic]*pubsub.Subscription)

	if err == nil {
		ip, port := p.GetIPPort()
		p.logger.Debug().Str("name", p.name).Str("address", fmt.Sprintf("%s:%s", ip, port)).
			Msg("libp2p node started successfully")
	}

	return err
}

// Stop stops the libp2p node.
func (p *P2PNode) Stop() error {
	p.Lock()
	defer p.Unlock()
	err := p.libP2PHost.Close()
	if err != nil {
		err = fmt.Errorf("could not stop node: %w", err)
	} else {
		p.logger.Debug().Str("name", p.name).Msg("libp2p node stopped successfully")
	}
	return err
}

// AddPeers adds other nodes as peers to this node by adding them to the node's peerstore and connecting to them
func (p *P2PNode) AddPeers(ctx context.Context, peers ...NodeAddress) error {
	p.Lock()
	defer p.Unlock()
	for _, peer := range peers {
		pInfo, err := GetPeerInfo(peer)
		if err != nil {
			return err
		}

		// Add the destination's peer multiaddress in the peerstore.
		// This will be used during connection and stream creation by libp2p.
		p.libP2PHost.Peerstore().AddAddrs(pInfo.ID, pInfo.Addrs, peerstore.PermanentAddrTTL)

		err = p.libP2PHost.Connect(ctx, pInfo)
		if err != nil {
			return err
		}
	}
	return nil
}

// CreateStream returns an existing stream connected to n if it exists or adds node n as a peer and creates a new stream with it
func (p *P2PNode) CreateStream(ctx context.Context, n NodeAddress) (network.Stream, error) {

	// Get the PeerID
	peerID, err := GetPeerID(n.Name)
	if err != nil {
		return nil, err
	}

	stream := FindOutboundStream(p.libP2PHost, peerID, FlowLibP2PProtocolID)

	// if existing stream found return it
	if stream != nil {
		p.logger.Debug().Str("protocol", string(stream.Protocol())).
			Str("stream_direction", DirectionToString(stream.Stat().Direction)).
			Str("connection_direction", DirectionToString(stream.Conn().Stat().Direction)).
			Msg("found existing stream")
		return stream, nil
	}

	// Add node address as a peer
	err = p.AddPeers(ctx, n)
	if err != nil {
		return nil, err
	}

	// Open libp2p Stream with the remote peer (will use an existing TCP connection underneath)
	return p.libP2PHost.NewStream(ctx, peerID, FlowLibP2PProtocolID)
}

// GetPeerInfo generates the address of a Node/Peer given its address in a deterministic and consistent way.
// Libp2p uses the hash of the public key of node as its id (https://docs.libp2p.io/reference/glossary/#multihash)
// Since the public key of a node may not be available to other nodes, for now a simple scheme of naming nodes can be
// used e.g. "node1, node2,... nodex" to helps nodes address each other.
// An MD5 hash of such of the node Name is used as a seed to a deterministic crypto algorithm to generate the
// public key from which libp2p derives the node id
func GetPeerInfo(p NodeAddress) (peer.AddrInfo, error) {
	addr := getLocationMultiaddrString(p)
	maddr, err := multiaddr.NewMultiaddr(addr)
	if err != nil {
		return peer.AddrInfo{}, err
	}
	id, err := GetPeerID(p.Name)
	if err != nil {
		return peer.AddrInfo{}, err
	}
	pInfo := peer.AddrInfo{ID: id, Addrs: []multiaddr.Multiaddr{maddr}}
	return pInfo, err
}

// GetIPPort returns the IP and Port the libp2p node is listening on.
func (p *P2PNode) GetIPPort() (ip string, port string) {
	for _, a := range p.libP2PHost.Network().ListenAddresses() {
		if ip, e := a.ValueForProtocol(multiaddr.P_IP4); e == nil {
			if p, e := a.ValueForProtocol(multiaddr.P_TCP); e == nil {
				return ip, p
			}
		}
	}
	return "", ""
}

// Subscribe subscribes the node to the given topic. When a message is received for the topic, the callback is called
// with the message payload
// Currently only one subscriber is allowed per topic.
// A node will receive its own published messages.
func (p *P2PNode) Subscribe(ctx context.Context, topic FlowTopic, callback func([]byte)) error {
	p.Lock()
	defer p.Unlock()
	// Check if the topic has been already created and is in the cache
	tp, found := p.topics[topic]
	var err error
	if !found {
		tp, err = p.ps.Join(string(topic))
		if err != nil {
			return errors.Wrapf(err, "failed to register for topic %s", string(topic))
		}
		p.topics[topic] = tp
	}

	// Create a new subscription
	s, err := tp.Subscribe()
	if err != nil {
		return err
	}
	// Add the subscription to the cache
	p.subs[topic] = s
	go pubSubHandler(ctx, s, callback, p.logger)

	p.logger.Debug().Str("topic", string(topic)).Str("name", p.name).Msg("subscribed to topic")
	return err
}

// pubSubHandler receives the messages for a subscriber and calls the registered call back
func pubSubHandler(c context.Context, s *pubsub.Subscription, callback func([]byte), l zerolog.Logger) error {
	for {
		msg, err := s.Next(c)
		if err != nil {
			return err
		}
		callback(msg.Data)
	}
}

// UnSubscribe cancels the subscriber and closes the topic.
func (p *P2PNode) UnSubscribe(topic FlowTopic) error {
	p.Lock()
	defer p.Unlock()
	// Remove the Subscriber from the cache
	s := p.subs[topic]
	if s != nil {
		s.Cancel()
		p.subs[topic] = nil
		delete(p.subs, topic)
	}

	tp, found := p.topics[topic]
	if !found {
		err := fmt.Errorf("topic %s not subscribed to", topic)
		return err
	}

	err := tp.Close()
	if err != nil {
		err = errors.Wrapf(err, "unable to close topic %s", string(topic))
		return err
	}
	p.topics[topic] = nil
	delete(p.topics, topic)

	p.logger.Debug().Str("topic", string(topic)).Str("name", p.name).Msg("unsubscribed from topic")
	return err
}

// Publish publishes the given payload on the topic
func (p *P2PNode) Publish(ctx context.Context, t FlowTopic, data []byte) error {
	ps, found := p.topics[t]
	if !found {
		return fmt.Errorf("topic not found")
	}
	return ps.Publish(ctx, data)
}

// GetLocationMultiaddr returns a Multiaddress string (https://docs.libp2p.io/concepts/addressing/) given a node address
func getLocationMultiaddrString(id NodeAddress) string {
	return fmt.Sprintf("/ip4/%s/tcp/%s", id.IP, id.Port)
}
