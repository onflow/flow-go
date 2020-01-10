// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package middleware

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
	"github.com/dapperlabs/flow-go/network/gossip/libp2p"
)

// Middleware handles the input & output on the direct connections we have to
// our neighbours on the peer-to-peer network.
type Middleware struct {
	sync.Mutex
	log        zerolog.Logger
	codec      network.Codec
	ov         libp2p.Overlay
	slots      chan struct{} // semaphore for outgoing connection slots
	conns      map[flow.Identifier]*WriteConnection
	wg         *sync.WaitGroup
	libP2PNode *libp2p.P2PNode
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

	nodeAddress := libp2p.NodeAddress{Name: flowID.String(), IP: ip, Port: port}
	p2p := &libp2p.P2PNode{}

	// create the node entity and inject dependencies & config
	m := &Middleware{
		log:        log,
		codec:      codec,
		slots:      make(chan struct{}, conns),
		conns:      make(map[flow.Identifier]*WriteConnection),
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
func (m *Middleware) Start(ov libp2p.Overlay) {
	m.ov = ov
	m.wg.Add(1)
	go m.rotate()
}

// Stop will end the execution of the middleware and wait for it to end.
func (m *Middleware) Stop() {
	close(m.stop)
	m.libP2PNode.Stop()
	for _, conn := range m.conns {
		conn.stop()
	}
	//m.wg.Wait()
}

// Send will try to send the given message to the given peer.
func (m *Middleware) Send(nodeID flow.Identifier, msg interface{}) error {
	m.Lock()
	defer m.Unlock()

	// get the conn from our list
	conn, ok := m.conns[nodeID]
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

func (m *Middleware) createOutboundMessage(msg interface{}) *libp2p.Message {
	// Compose the message payload
	message := &libp2p.Message{
		SenderID: m.me[:],
		Event:    msg.([]byte),
	}
	return message
}

func (m *Middleware) createInboundMessage(msg *libp2p.Message) (*flow.Identifier, interface{}, error) {

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
	if m.exists(flowIdentity.NodeID) {
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
	nodeAddress := libp2p.NodeAddress{Name: flowIdentity.NodeID.String(), IP: ip, Port: port}
	// Add it as a peer
	stream, err := m.libP2PNode.CreateStream(context.Background(), nodeAddress)
	if err != nil {
		log.Error().Err(err).Msgf("failed to create stream for %s", nodeAddress.Name)
		return
	}

	// make sure we close the stream once the handling is done
	defer stream.Close()

	// this is a blocking call so that the deferred cleanups run only after we are
	// done handling this peer
	m.handleOutgoing(flowIdentity.NodeID, stream)
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

func (m *Middleware) handleOutgoing(flowID flow.Identifier, s libp2pnetwork.Stream) {

	log := m.log.With().
		Str("local_addr", s.Conn().LocalPeer().String()).
		Str("remote_addr", s.Conn().RemotePeer().String()).
		Logger()

	// create the write connection handler
	conn := NewWriteConnection(log, s)

	// cache the connection against the node id
	m.add(flowID, conn)
	defer m.remove(flowID)

	log.Info().Msg("connection established")

	// kick off the send loop
	conn.SendLoop()
}

// release will release one resource on the given semaphore.
func (m *Middleware) release(slots chan struct{}) {
	<-slots
}

// add will add the given conn with the given address to our list in a
// concurrency-safe manner.
func (m *Middleware) add(nodeID flow.Identifier, conn *WriteConnection) {
	m.Lock()
	defer m.Unlock()
	m.conns[nodeID] = conn
}

// exists will check if the connection already exists in a concurrency-safe manner.
func (m *Middleware) exists(nodeID flow.Identifier) bool {
	m.Lock()
	defer m.Unlock()
	_, ok := m.conns[nodeID]
	return ok
}

// remove will remove the connection with the given nodeID from the list in
// a concurrency-safe manner.
func (m *Middleware) remove(nodeID flow.Identifier) {
	m.Lock()
	defer m.Unlock()
	delete(m.conns, nodeID)
}
