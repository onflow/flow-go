// Package libp2p encapsulates the libp2p library
package libp2p

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"sync"
	"time"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/gogo/protobuf/proto"

	flownetwork "github.com/dapperlabs/flow-go/network"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/protocol"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-tcp-transport"
	"github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

// A unique Libp2p protocol ID for Flow (https://docs.libp2p.io/concepts/protocols/)
// All nodes communicate with each other using this protocol
const (
	FlowLibP2PProtocolID protocol.ID = "/flow/0.0.1"
)

// NodeAddress is used to define a libp2p node
type NodeAddress struct {
	// name is the friendly node name e.g. "node1" (not to be confused with the libp2p node id)
	name string
	ip   string
	port string
}

// P2PNode manages the the libp2p node.
type P2PNode struct {
	sync.Mutex
	name       string                             // friendly human readable name of the node
	libP2PHost host.Host                          // reference to the libp2p host (https://godoc.org/github.com/libp2p/go-libp2p-core/host)
	logger     zerolog.Logger                     // for logging
	ps         *pubsub.PubSub                     // the reference to the pubsub instance
	topics     map[FlowTopic]*pubsub.Topic        // map of a topic string to an actual topic instance
	subs       map[FlowTopic]*pubsub.Subscription // map of a topic string to an actual subscription
	engines    map[uint8]flownetwork.Engine       // map of engine id to engine instances
	streams    map[uint8]network.Stream           // map of engine id to libp2p streams

	//TODO abstract this out in a different class
	inbound chan interface{}
}

// Start starts a libp2p node on the given address.
func (p *P2PNode) Start(ctx context.Context, n NodeAddress, logger zerolog.Logger) error {
	p.Lock()
	defer p.Unlock()
	p.name = n.name
	p.logger = logger
	addr := getLocationMultiaddrString(n)
	sourceMultiAddr, err := multiaddr.NewMultiaddr(addr)
	if err != nil {
		return err
	}

	key, err := GetPublicKey(n.name)
	if err != nil {
		err = errors.Wrapf(err, "could not generate public key for %s", p.name)
		return err
	}

	// libp2p.New constructs a new libp2p Host.
	// Other options can be added here.
	host, err := libp2p.New(
		ctx,
		libp2p.ListenAddrs(sourceMultiAddr),
		libp2p.NoSecurity,
		libp2p.Identity(key),
		libp2p.Transport(tcp.NewTCPTransport), // the default transport unnecessarily brings in a websocket listener
	)

	time.Sleep(time.Second * 2)
	p.libP2PHost = host

	// Set the callback to use for an incoming peer message
	host.SetStreamHandler(FlowLibP2PProtocolID, p.handleStream)

	// Creating a new PubSub instance of the type GossipSub
	p.ps, err = pubsub.NewGossipSub(ctx, p.libP2PHost)

	if err != nil {
		return errors.Wrapf(err, "unable to start pubsub %s", p.name)
	}

	p.topics = make(map[FlowTopic]*pubsub.Topic)
	p.subs = make(map[FlowTopic]*pubsub.Subscription)
	p.engines = make(map[uint8]flownetwork.Engine)

	if err == nil {
		p.logger.Debug().Str("name", p.name).Msg("libp2p node started successfully")
	}

	return err
}

func (p *P2PNode) Send(ctx context.Context, peer NodeAddress, engineID uint8, msg interface{}) error {
	peerInfo, err := GetPeerInfo(peer)
	if err != nil {
		return err
	}
	stream, err := p.libP2PHost.NewStream(ctx, peerInfo.ID)
	if err != nil {
		return err
	}

	// TODO: Add translation of flow network id to libp2p name or make them the same
	var senderID [32]byte
	copy(senderID[:], []byte(p.name))
	message := &Message{SenderID: make([]byte, len(senderID)), Event: msg.([]byte), EngineID: uint32(engineID)}
	b, err := proto.Marshal(message)
	if err != nil {
		return err
	}

	// TODO: Do something with the number of bytes written
	_, err = stream.Write(b)
	if err != nil {
		return err
	}
	err = stream.Close()
	if err != nil {
		return err
	}
	return nil
}

// Stop stops the libp2p node.
func (p *P2PNode) Stop() error {
	p.Lock()
	defer p.Unlock()
	err := p.libP2PHost.Close()
	if err == nil {
		p.logger.Debug().Str("name", p.name).Msg("libp2p node stopped successfully")
	}
	return err
}

// AddPeers adds other nodes as peers to this node by adding them to the node's peerstore and connecting to them
func (p *P2PNode) AddPeers(ctx context.Context, peers []NodeAddress) error {
	p.Lock()
	defer p.Unlock()
	for _, peer := range peers {
		pInfo, err := GetPeerInfo(peer)
		if err != nil {
			return err
		}

		for _, m := range pInfo.Addrs {
			fmt.Printf(" %s\n", m.String())
		}
		// Add the destination's peer multiaddress in the peerstore.
		// This will be used during connection and stream creation by libp2p.
		fmt.Printf(" Adding node %s to %s\n", pInfo.ID, p.libP2PHost.ID())
		p.libP2PHost.Peerstore().AddAddrs(pInfo.ID, pInfo.Addrs, peerstore.PermanentAddrTTL)
		fmt.Printf(" Added node %s to %s\n", pInfo.ID, p.libP2PHost.ID())


		fmt.Printf(" Connecting node %s to %s\n", pInfo.ID, p.libP2PHost.ID())
		err = p.libP2PHost.Connect(ctx, pInfo)
		if err != nil {
			return err
		}
		fmt.Printf(" Connected node %s to %s\n", pInfo.ID, p.libP2PHost.ID())
	}
	return nil
}

// GetPeerInfo generates the address of a Node/Peer given its address in a deterministic and consistent way.
// Libp2p uses the hash of the public key of node as its id (https://docs.libp2p.io/reference/glossary/#multihash)
// Since the public key of a node may not be available to other nodes, for now a simple scheme of naming nodes can be
// used e.g. "node1, node2,... nodex" to helps nodes address each other.
// An MD5 hash of such of the node name is used as a seed to a deterministic crypto algorithm to generate the
// public key from which libp2p derives the node id
func GetPeerInfo(p NodeAddress) (peer.AddrInfo, error) {
	addr := getLocationMultiaddrString(p)
	maddr, err := multiaddr.NewMultiaddr(addr)
	if err != nil {
		return peer.AddrInfo{}, err
	}
	id, err := GetPeerID(p.name)
	if err != nil {
		return peer.AddrInfo{}, err
	}
	pInfo := peer.AddrInfo{ID: id, Addrs: []multiaddr.Multiaddr{maddr}}
	return pInfo, err
}

// GetIPPort returns the IP and Port the libp2p node is listening on.
func (p *P2PNode) GetIPPort() (ip string, port string) {
	p.Lock()
	defer p.Unlock()
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

// Register will register the given engine with the given unique engine engineID,
// returning a conduit to directly submit messages to the message bus of the
// engine.
func (p *P2PNode) Register(engineID uint8, engine flownetwork.Engine) (flownetwork.Conduit, error) {
	p.Lock()
	defer p.Unlock()
	// check if the engine engineID is already taken
	_, found := p.engines[engineID]
	if found {
		return nil, errors.Errorf("engine already registered (%d)", engine)
	}
	// register engine with provided engineID
	p.engines[engineID] = engine

	// create the conduit
	conduit := &Conduit{
		engineID: engineID,
		submit:   p.submit,
	}
	return conduit, nil
}

// submit will submit the given event for the given engine to the overlay layer
// for processing; it is used by engines through conduits.
// The Target needs to be added as a peer before submitting the message.
func (p *P2PNode) submit(engineID uint8, event interface{}, targetIDs ...flow.Identifier) error {
	for _, t := range targetIDs {
		// TODO: Yet to figure out the whole flow identifier to libp2p id mapping.

		// Translate the target flow ID to the libp2p peer id
		// Remove the extra 0s at the end
		flowID := bytes.Trim(t[:], "\x00")
		// Translate the bytes to the node name
		flowIDStr := string(flowID)
		// Get the libp2p id from the node name
		peerID, err := GetPeerID(flowIDStr)
		if err != nil {
			return err
		}

		var senderID [32]byte
		// Convert node name to self to sender ID
		copy(senderID[:], p.name)
		// Compose the message payload
		message := &Message{SenderID: senderID[:], Event: event.([]byte), EngineID: uint32(engineID)}
		// Get the ProtoBuf representation of the message
		b, err := proto.Marshal(message)
		if err != nil {
			return err
		}

		// Open libp2p Stream with the remote peer (will use an existing TCP connection underneath)
		stream, err := p.libP2PHost.NewStream(context.Background(), peerID, FlowLibP2PProtocolID)
		if err != nil {
			return err
		}

		// Send the message using the stream
		// TODO: Do something with the return value of number of bytes written
		_, err = stream.Write(b)
		if err != nil {
			return err
		}

		// Debug log the message length
		p.logger.Debug().Str("peer", stream.Conn().RemotePeer().String()).
			Str("message", message.String()).Int("length", len(b)).
			Msg("sent message")

		// Close the stream
		err = stream.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// handleStream is the callback that gets called for each remote peer that makes a direct 1-1 connection
func (p *P2PNode) handleStream(s network.Stream) {
	p.logger.Debug().Str("peer", s.Conn().RemotePeer().String()).Msg("received a new incoming stream")
	go p.readData(s)
	// TODO: Running with a receive-only (unidirectional) stream for now
	// go p.writeData(s)
}

// readData reads the data from the remote peer using the stream
func (p *P2PNode) readData(s network.Stream) {
	for {
		// TODO: implement length-prefix framing to delineate protobuf message if exchanging more than one message
		// (protobuf has no inherent delimiter)
		// Read incoming data into a buffer
		buff, err := ioutil.ReadAll(s)
		if err != nil {
			p.logger.Error().Str("peer", s.Conn().RemotePeer().String()).Err(err)
			s.Close()
			return
		}

		// ioutil.ReadAll continues to read even after an EOF is encountered.
		// Close connection and return in that case (This is not an error)
		if len(buff) <= 0 {
			s.Close()
			return
		}
		p.logger.Debug().Str("peer", s.Conn().RemotePeer().
			String()).Bytes("message", buff).Int("length", len(buff)).
			Msg("received message")

		// Unmarshal the buff to a message
		message := &Message{}
		err = proto.Unmarshal(buff, message)
		if err != nil {
			p.logger.Error().Str("peer", s.Conn().RemotePeer().String()).Err(err)
			return
		}

		// Extract sender id
		if len(message.SenderID) < 32 {
			p.logger.Debug().Str("peer", s.Conn().RemotePeer().String()).
				Bytes("sender", message.SenderID).
				Msg(" invalid sender id")
			return
		}
		var senderID [32]byte
		copy(senderID[:], message.SenderID)

		// Extract engine id and find the registered engine
		en, found := p.engines[uint8(message.EngineID)]
		if !found {
			p.logger.Debug().Str("peer", s.Conn().RemotePeer().String()).
				Uint8("engine", uint8(message.EngineID)).
				Msg(" dropping message since no engine to receive it was found")
			return
		}
		// call the engine with the message payload
		err = en.Process(senderID, message.Event)
		if err != nil {
			p.logger.Error().
				Uint8("engineid", uint8(message.EngineID)).
				Str("peer", s.Conn().RemotePeer().String()).Err(err)
			return
		}
	}
}

// GetLocationMultiaddr returns a Multiaddress string (https://docs.libp2p.io/concepts/addressing/) given a node address
func getLocationMultiaddrString(id NodeAddress) string {
	return fmt.Sprintf("/ip4/%s/tcp/%s", id.ip, id.port)
}
