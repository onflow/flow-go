// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package libp2p

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"

	libp2pnetwork "github.com/libp2p/go-libp2p-core/network"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p/message"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p/middleware"
)

// Middleware handles the input & output on the direct connections we have to
// our neighbours on the peer-to-peer network.
type Middleware struct {
	sync.Mutex
	log        zerolog.Logger
	codec      network.Codec
	ov         middleware.Overlay
	cc         *ConnectionCache
	wg         *sync.WaitGroup
	libP2PNode *P2PNode
	stop       chan struct{}
	me         flow.Identifier
	host       string
	port       string
}

// NewMiddleware creates a new middleware instance with the given config and using the
// given codec to encode/decode messages to our peers.
func NewMiddleware(log zerolog.Logger, codec network.Codec, address string, flowID flow.Identifier) (*Middleware, error) {
	ip, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}

	p2p := &P2PNode{}

	// create the node entity and inject dependencies & config
	m := &Middleware{
		log:        log,
		codec:      codec,
		cc:         NewConnectionCache(),
		libP2PNode: p2p,
		wg:         &sync.WaitGroup{},
		stop:       make(chan struct{}),
		me:         flowID,
		host:       ip,
		port:       port,
	}

	return m, err
}

// Me returns the flow identifier of the this middleware
func (m *Middleware) Me() flow.Identifier {
	return m.me
}

// GetIPPort returns the ip address and port number associated with the middleware
func (m *Middleware) GetIPPort() (string, string) {
	return m.libP2PNode.GetIPPort()
}

// Start will start the middleware.
func (m *Middleware) Start(ov middleware.Overlay) error {

	m.ov = ov

	// create a discovery object to help libp2p discover peers
	d := NewDiscovery(m.log, m.ov, m.me)

	// create PubSub options for libp2p to use the discovery object
	psOption := pubsub.WithDiscovery(d)

	nodeAddress := NodeAddress{Name: m.me.String(), IP: m.host, Port: m.port}

	// start the libp2p node
	err := m.libP2PNode.Start(context.Background(), nodeAddress, m.log, m.handleIncomingStream, psOption)

	if err != nil {
		return fmt.Errorf("failed to start libp2p node: %w", err)
	}

	// the ip,port may change after libp2p has been started. e.g. 0.0.0.0:0 would change to an actual IP and port
	m.host, m.port = m.libP2PNode.GetIPPort()

	return nil
}

// Stop will end the execution of the middleware and wait for it to end.
func (m *Middleware) Stop() {
	close(m.stop)

	// Stop all the connections
	for _, conn := range m.cc.GetAll() {
		conn.stop()
	}

	// Stop libp2p
	err := m.libP2PNode.Stop()
	if err != nil {
		log.Error().Err(err).Msg("stopping failed")
	} else {
		log.Debug().Msg("node stopped successfully")
	}
	m.wg.Wait()
}

// Send will try to send the given message to the given peer.
func (m *Middleware) Send(targetID flow.Identifier, msg interface{}) error {
	m.Lock()
	defer m.Unlock()
	found, stale := false, false
	var conn *WriteConnection

	log := m.log.With().Str("nodeid", targetID.String()).Logger()

	if conn, found = m.cc.Get(targetID); found {
		// check if the peer is still running
		select {
		case <-conn.done:
			// connection found to be stale; replace with a new one
			log.Debug().Msg("existing connection already closed ")
			stale = true
			conn = nil
			m.cc.Remove(targetID)
		default:
			log.Debug().Msg("reusing existing connection")
		}
	} else {
		log.Debug().Str("nodeid", targetID.String()).Msg("connection not found, creating one")
	}

	if !found || stale {

		// get an identity to connect to. The identity provides the destination TCP address.
		idsMap, err := m.ov.Identity()
		if err != nil {
			return fmt.Errorf("could not get identities: %w", err)
		}
		flowIdentity, found := idsMap[targetID]
		if !found {
			return fmt.Errorf("could not get identity for %s: %w", targetID.String(), err)
		}

		// create new connection
		conn, err = m.connect(flowIdentity.NodeID.String(), flowIdentity.Address)
		if err != nil {
			return fmt.Errorf("could not create new connection for %s: %w", targetID.String(), err)
		}

		// cache the connection against the node id
		m.cc.Add(flowIdentity.NodeID, conn)

		// kick-off a go routine (one for each outbound connection)
		m.wg.Add(1)
		go m.handleOutboundConnection(flowIdentity.NodeID, conn)

	}

	// send the message if connection still valid
	select {
	case <-conn.done:
		return errors.Errorf("connection has closed (node_id: %s)", targetID.String())
	default:
		switch msg := msg.(type) {
		case *message.Message:
			// Write message to outbound channel only if it is of the correct type
			// TODO: check if outbound channel is already closed
			conn.outbound <- msg
		default:
			err := errors.Errorf("middleware received invalid message type (%T)", msg)
			return err
		}
		return nil
	}
}

// connect creates a new connection
func (m *Middleware) connect(flowID string, address string) (*WriteConnection, error) {

	log := m.log.With().Str("targetid", flowID).Str("address", address).Logger()

	ip, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("could not parse address %s:%v", address, err)
	}

	// Create a new NodeAddress
	nodeAddress := NodeAddress{Name: flowID, IP: ip, Port: port}

	// Create a stream for it
	stream, err := m.libP2PNode.CreateStream(context.Background(), nodeAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to create stream for %s:%v", nodeAddress.Name, err)
	}

	clog := m.log.With().
		Str("local_addr", stream.Conn().LocalPeer().String()).
		Str("remote_addr", stream.Conn().RemotePeer().String()).
		Logger()

	// create the write connection handler
	conn := NewWriteConnection(clog, stream)

	log.Info().Msg("connection established")

	return conn, nil
}

func (m *Middleware) handleOutboundConnection(targetID flow.Identifier, conn *WriteConnection) {
	defer m.wg.Done()
	// Remove the conn from the cache when done
	defer m.cc.Remove(targetID)
	// make sure we close the stream once the handling is done
	defer conn.stream.Close()
	// kick off the send loop
	conn.SendLoop(m.stop)
}

// handleIncomingStream handles an incoming stream from a remote peer
// this is a blocking call, so that the deferred resource cleanup happens after
// we are done handling the connection
func (m *Middleware) handleIncomingStream(s libp2pnetwork.Stream) {
	m.wg.Add(1)
	defer m.wg.Done()

	log := m.log.With().
		Str("local_addr", s.Conn().LocalPeer().String()).
		Str("remote_addr", s.Conn().RemotePeer().String()).
		Logger()

	// initialize the encoder/decoder and create the connection handler
	conn := NewReadConnection(log, s)

	// make sure we close the connection when we are done handling the peer
	defer conn.stop()

	log.Info().Msg("incoming connection established")

	// start processing messages in the background
	go conn.ReceiveLoop()

	// process incoming messages for as long as the peer is running
ProcessLoop:
	for {
		select {
		case <-m.stop:
			m.log.Info().Msg("exiting process loop: middleware stops")
			break ProcessLoop
		case <-conn.done:
			m.log.Info().Msg("exiting process loop: connection stops")
			break ProcessLoop
		case msg := <-conn.inbound:
			m.processMessage(msg)
			continue ProcessLoop
		}
	}

	log.Info().Msg("middleware closed the connection")
}

// Subscribe will subscribe the middleware for a topic with the same name as the channelID
func (m *Middleware) Subscribe(channelID uint8) error {
	// A Flow ChannelID becomes the topic ID in libp2p.
	s, err := m.libP2PNode.Subscribe(context.Background(), strconv.Itoa(int(channelID)))
	if err != nil {
		return fmt.Errorf("failed to subscribe for channel %d: %w", channelID, err)
	}
	rs := NewReadSubscription(m.log, s)
	go rs.ReceiveLoop()

	// add to waitgroup to wait for the inbound subscription go routine during stop
	m.wg.Add(1)
	go m.handleInboundSubscription(rs)
	return nil
}

// handleInboundSubscription reads the messages from the channel written to by readsSubscription and processes them
func (m *Middleware) handleInboundSubscription(rs *ReadSubscription) {
	defer m.wg.Done()
	// process incoming messages for as long as the peer is running
SubscriptionLoop:
	for {
		select {
		case <-m.stop:
			// middleware stops
			m.log.Info().Msg("exiting subscription loop: middleware stops")
			break SubscriptionLoop
		case <-rs.done:
			// subscription stops
			m.log.Info().Msg("exiting subscription loop: connection stops")
			break SubscriptionLoop
		case msg := <-rs.inbound:
			m.processMessage(msg)
			continue SubscriptionLoop
		}
	}
}

// processMessager processes a message and eventuall passes it to the overlay
func (m *Middleware) processMessage(msg *message.Message) {
	nodeID, err := getSenderID(msg)
	if err != nil {
		log.Error().Err(err).Msg("could not extract sender ID")
	}
	if nodeID == m.me {
		return
	}
	err = m.ov.Receive(nodeID, msg)
	if err != nil {
		log.Error().Err(err).Msg("could not deliver payload")
	}
}

// Publish publishes the given payload on the topic
func (m *Middleware) Publish(topic string, msg interface{}) error {
	switch msg := msg.(type) {
	case *message.Message:

		// convert the message to bytes to be put on the wire.
		data, err := msg.Marshal()
		if err != nil {
			return fmt.Errorf("failed to marshal the message: %w", err)
		}

		// publish the bytes on the topic
		// pubsub.GossipSubDlo is the minimal number of peer connections that libp2p will maintain
		// for this node.
		// TODO: specify a minpeers that makes more sense for Flow (this however requires GossipSubDlo to be adjusted.
		err = m.libP2PNode.Publish(context.Background(), topic, data, pubsub.GossipSubDlo)
		if err != nil {
			return fmt.Errorf("failed to publish the message: %w", err)
		}
	default:
		err := fmt.Errorf("middleware received invalid message type (%T)", msg)
		return err
	}
	return nil
}

func getSenderID(msg *message.Message) (flow.Identifier, error) {
	// Extract sender id
	if len(msg.OriginID) < 32 {
		err := fmt.Errorf("invalid sender id")
		return flow.ZeroID, err
	}
	var id flow.Identifier
	copy(id[:], msg.OriginID)
	return id, nil
}
