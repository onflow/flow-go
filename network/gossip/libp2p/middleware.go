// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package libp2p

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	libp2pnetwork "github.com/libp2p/go-libp2p-core/network"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p/middleware"
)

// Middleware handles the input & output on the direct connections we have to
// our neighbours on the peer-to-peer network.
type Middleware struct {
	sync.Mutex
	log        zerolog.Logger
	codec      network.Codec
	ov         middleware.Overlay
	slots      chan struct{} // semaphore for outgoing connection slots
	cc         *ConnectionCache
	wg         *sync.WaitGroup
	libP2PNode *P2PNode
	stop       chan struct{}
	me         flow.Identifier
}

// New creates a new middleware instance with the given config and using the
// given codec to encode/decode messages to our peers.
func New(log zerolog.Logger, codec network.Codec, conns uint, address string, flowID flow.Identifier) (*Middleware, error) {
	ip, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}

	nodeAddress := NodeAddress{Name: flowID.String(), IP: ip, Port: port}
	p2p := &P2PNode{}

	// create the node entity and inject dependencies & config
	m := &Middleware{
		log:        log,
		codec:      codec,
		slots:      make(chan struct{}, conns),
		cc:         NewConnectionCache(),
		libP2PNode: p2p,
		wg:         &sync.WaitGroup{},
		stop:       make(chan struct{}),
		me:         flowID,
	}

	// Start the libp2p node
	err = p2p.Start(context.Background(), nodeAddress, log, m.handleIncomingStream)

	return m, err
}

// Start will start the middleware.
func (m *Middleware) Start(ov middleware.Overlay) {
	m.ov = ov
	m.wg.Add(1)
	go m.rotate()
}

// Stop will end the execution of the middleware and wait for it to end.
func (m *Middleware) Stop() {
	close(m.stop)
	m.libP2PNode.Stop()
	for _, conn := range m.cc.GetAll() {
		conn.stop()
	}
	m.wg.Wait()
}

// Send will try to send the given message to the given peer.
func (m *Middleware) Send(nodeID flow.Identifier, msg interface{}) error {
	m.Lock()
	defer m.Unlock()

	// get the conn from the conn cache
	conn, ok := m.cc.Get(nodeID)
	if !ok {
		return errors.Errorf("connection not found (node_id: %s)", nodeID)
	}

	// check if the peer is still running
	select {
	case <-conn.done:
		return errors.Errorf("connection already closed (node_id: %s)", nodeID)
	default:
	}

	pmsg := m.createOutboundMessage(msg)

	// whichever comes first, sending the message or ending the provided context
	select {
	case <-conn.done:
		return errors.Errorf("connection has closed (node_id: %s)", nodeID)
	case conn.outbound <- pmsg:
		return nil
	}
}

func (m *Middleware) createOutboundMessage(msg interface{}) *Message {
	// Compose the message payload
	message := &Message{
		SenderID: m.me[:],
		Event:    msg.([]byte),
	}
	return message
}

func (m *Middleware) createInboundMessage(msg *Message) (*flow.Identifier, interface{}, error) {

	// Extract sender id
	if len(msg.SenderID) < 32 {
		m.log.Debug().
			Bytes("sender", msg.SenderID).
			Msg(" invalid sender id")
		err := fmt.Errorf("invalid sender id")
		return nil, nil, err
	}
	var senderID [32]byte
	copy(senderID[:], msg.SenderID)
	var id flow.Identifier
	id = senderID
	return &id, msg.Event, nil
}

// rotate periodically creates connection to peers
func (m *Middleware) rotate() {
	defer m.wg.Done()

Loop:
	for {
		select {

		// for each free connection slot, we create a new connection
		case m.slots <- struct{}{}:

			// launch connection attempt
			m.wg.Add(1)
			go m.connect()

			// TODO: add proper rate limiter
			time.Sleep(time.Second)

		case <-m.stop:
			break Loop
		}
	}
}

// connect creates an outbound stream
func (m *Middleware) connect() {
	defer m.wg.Done()

	log := m.log.With().Logger()

	// make sure we free up the connection slot once we drop the peer
	defer m.release(m.slots)

	// get an identity to connect to
	flowIdentity, err := m.ov.Identity()
	if err != nil {
		log.Error().Err(err).Msg("could not get flow ID")
		return
	}

	// If this node is already added, noop
	if m.cc.Exists(flowIdentity.NodeID) {
		log.Debug().Str("flowIdentity", flowIdentity.String()).Msg("node already added")
		return
	}

	log = log.With().Str("flowIdentity", flowIdentity.String()).Str("address", flowIdentity.Address).Logger()

	ip, port, err := net.SplitHostPort(flowIdentity.Address)
	if err != nil {
		log.Error().Err(err).Msg("could not parse address")
		return
	}
	// Create a new NodeAddress
	nodeAddress := NodeAddress{Name: flowIdentity.NodeID.String(), IP: ip, Port: port}
	// Add it as a peer
	stream, err := m.libP2PNode.CreateStream(context.Background(), nodeAddress)
	if err != nil {
		log.Error().Err(err).Msgf("failed to create stream for %s", nodeAddress.Name)
		return
	}

	// make sure we close the stream once the handling is done
	defer stream.Close()

	clog := m.log.With().
		Str("local_addr", stream.Conn().LocalPeer().String()).
		Str("remote_addr", stream.Conn().RemotePeer().String()).
		Logger()

	// create the write connection handler
	conn := NewWriteConnection(clog, stream)

	// cache the connection against the node id and remove it when done
	m.cc.Add(flowIdentity.NodeID, conn)
	defer m.cc.Remove(flowIdentity.NodeID)

	log.Info().Msg("connection established")

	// kick off the send loop
	conn.SendLoop()
}

// handleIncomingStream handles an incoming stream from a remote peer
// this is a blocking call, so that the deferred resource cleanup happens after
// we are done handling the connection
func (m *Middleware) handleIncomingStream(s libp2pnetwork.Stream) {
	m.wg.Add(1)
	defer m.wg.Done()

	// make sure we close the connection when we are done handling the peer
	defer s.Close()

	log := m.log.With().
		Str("local_addr", s.Conn().LocalPeer().String()).
		Str("remote_addr", s.Conn().RemotePeer().String()).
		Logger()

	// initialize the encoder/decoder and create the connection handler
	conn := NewReadConnection(log, s)

	log.Info().Msg("incoming connection established")

	// start processing messages in the background
	go conn.ReceiveLoop()

	// process incoming messages for as long as the peer is running
ProcessLoop:
	for {
		select {
		case <-conn.done:
			log.Info().Msg("breaking loop")
			break ProcessLoop
		case msg := <-conn.inbound:
			log.Info().Msg("got message")
			nodeID, payload, err := m.createInboundMessage(msg)
			if err != nil {
				log.Error().Err(err).Msg("could not extract payload")
				continue ProcessLoop
			}
			err = m.ov.Receive(*nodeID, payload)
			if err != nil {
				log.Error().Err(err).Msg("could not deliver payload")
				continue ProcessLoop
			}
		}
	}

	log.Info().Msg("connection closed")
}

// release will release one resource on the given semaphore.
func (m *Middleware) release(slots chan struct{}) {
	<-slots
}
