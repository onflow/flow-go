// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package middleware

import (
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/pkg/codec"
	"github.com/dapperlabs/flow-go/pkg/naive/overlay"
)

// Middleware handles the input & output on the direct connections we have to
// our neighbours on the peer-to-peer network.
type Middleware struct {
	sync.Mutex
	log       zerolog.Logger
	codec     codec.Codec
	address   overlay.AddressCallback
	handshake overlay.HandshakeCallback
	cleanup   overlay.CleanupCallback
	receive   overlay.ReceiveCallback
	slots     chan struct{} // semaphore for outgoing connection slots
	extra     chan struct{} // semaphore for extra connection slots (in or out)
	shuffle   time.Duration
	heartbeat time.Duration
	conns     map[string]*Connection
}

// New creates a new middleware instance with the given config and using the
// given codec to encode/decode messages to our peers.
func New(cfg Config, log zerolog.Logger, codec codec.Codec) (*Middleware, error) {

	// initialize the listener so we can receive incoming connections
	ln, err := net.Listen("tcp", cfg.ListenAddress)
	if err != nil {
		return nil, errors.Wrapf(err, "could not listen on address (%s)", cfg.ListenAddress)
	}

	// create the node entity and inject dependencies & config
	m := &Middleware{
		log:       log,
		codec:     codec,
		address:   defaultAddress,
		handshake: defaultHandshake,
		cleanup:   defaultCleanup,
		receive:   defaultReceive,
		slots:     make(chan struct{}, cfg.MinConns),
		extra:     make(chan struct{}, cfg.MaxConns-cfg.MinConns),
		shuffle:   cfg.ShuffleInterval,
		heartbeat: cfg.HeartbeatInterval,
		conns:     make(map[string]*Connection),
	}

	// launch the rotator to rotate connections and the host to accept connections
	go m.rotate()
	go m.host(ln)

	return m, nil
}

// GetAddress registers a callback to provide addresses for new connections.
func (m *Middleware) GetAddress(address overlay.AddressCallback) {
	m.address = address
}

// RunHandshake registers a callback to handle handshakes for new connections.
func (m *Middleware) RunHandshake(handshake overlay.HandshakeCallback) {
	m.handshake = handshake
}

// RunCleanup registers a callback to handle dropped connections.
func (m *Middleware) RunCleanup(cleanup overlay.CleanupCallback) {
	m.cleanup = cleanup
}

// OnReceive registers a callback to handle new received messages.
func (m *Middleware) OnReceive(receive overlay.ReceiveCallback) {
	m.receive = receive
}

// Send will try to send the given message to the given peer.
func (m *Middleware) Send(peer string, msg interface{}) error {
	m.Lock()
	defer m.Unlock()

	// get the peer from our list
	c, ok := m.conns[peer]
	if !ok {
		return errors.Errorf("unknown node connection (%s)", peer)
	}

	// check if the peer is still running
	select {
	case <-c.done:
		return errors.Errorf("peer connection closed (%s)", peer)
	default:
	}

	// whichever comes first, sending the message or ending the provided context
	select {
	case <-c.done:
		return errors.Errorf("peer connection closed (%s)", peer)
	case c.outbound <- &msg:
		return nil
	}
}

func (m *Middleware) rotate() {
	for {
		select {

		// for each free connection slot, we create a new connection
		case m.slots <- struct{}{}:

			// TODO: add proper rate limiter
			time.Sleep(time.Second)

			// launch connection attempt
			go m.connect()

		// on each shuffle interval, we kill a random peer
		case <-time.After(m.shuffle):
			addresses := make([]string, 0, len(m.conns))
			for address := range m.conns {
				addresses = append(addresses, address)
			}
			address := addresses[rand.Intn(len(addresses))]
			conn := m.conns[address]
			close(conn.done)
		}
	}
}

func (m *Middleware) connect() {

	log := m.log.With().Logger()

	// make sure we free up the connection slop once we drop the peer
	defer m.release(m.slots)

	// get an address to connect to
	address, err := m.address()
	if err != nil {
		log.Error().Err(err).Msg("could not get address")
		return
	}

	log = log.With().Str("address", address).Logger()

	// create the new connection
	conn, err := net.Dial("tcp", address)
	if err != nil {
		log.Error().Err(err).Msg("could not dial address")
		return
	}

	// make sure we close the connection once the handling is done
	defer conn.Close()

	// this is a blocking call so that the defered cleanups run only after we are
	// done handling this peer
	m.handle(conn)
}

func (m *Middleware) host(ln net.Listener) {

	log := m.log.With().Str("listen_address", ln.Addr().String()).Logger()

ListenLoop:
	for {

		// accept the next waiting connection
		conn, err := ln.Accept()
		if err != nil {
			log.Error().Err(err).Msg("could not accept connection")
			break ListenLoop
		}

		// initialize the connection
		go m.init(conn)
	}
}

func (m *Middleware) init(conn net.Conn) {

	log := m.log.With().Str("address", conn.RemoteAddr().String()).Logger()

	// make sure we close the connection when we are done handling the peer
	defer conn.Close()

	// get a free connection slot and make sure to free it after we drop the peer
	select {
	case m.slots <- struct{}{}:
		defer m.release(m.slots)
	case m.extra <- struct{}{}:
		defer m.release(m.extra)
	default:
		log.Debug().Msg("no free connection slots, dropping connection")
		return
	}

	// this is a blocking call, so that the defered resource cleanup happens after
	// we are done handling the connection
	m.handle(conn)
}

func (m *Middleware) handle(conn net.Conn) {

	log := m.log.With().
		Str("local_addr", conn.LocalAddr().String()).
		Str("remote_addr", conn.RemoteAddr().String()).
		Logger()

	// initialize the encoder/decoder and create the connection handler
	enc := m.codec.NewEncoder(conn)
	dec := m.codec.NewDecoder(conn)
	connection := NewConnection(log, enc, dec)

	// execute the initial handshake
	peer, err := m.handshake(connection)
	if err != nil {
		log.Error().Err(err).Msg("could not execute handshake")
		return
	}

	log = log.With().Str("peer", peer).Logger()

	// register the peer with the returned peer ID
	m.add(peer, connection)
	defer m.remove(peer)

	log.Debug().Msg("peer handling starting")

	// start processing messages in the background
	connection.Process()

	// process incoming messages for as long as the peer is running
ProcessLoop:
	for {
		select {
		case <-connection.done:
			break ProcessLoop
		case msg := <-connection.inbound:
			err := m.receive(peer, msg)
			if err != nil {
				log.Error().Err(err).Msg("could not process received message")
			}
		}
	}

	log.Debug().Msg("peer handling stopped")
}

// release will release one resource on the given semaphore.
func (m *Middleware) release(slots chan struct{}) {
	<-slots
}

// add will add the given connection with the given address to our list in a
// concurrency-safe manner.
func (m *Middleware) add(address string, conn *Connection) {
	m.Lock()
	defer m.Unlock()
	m.conns[address] = conn
}

// remove will remove the connection with the given address from the list in
// a concurrency-safe manner.
func (m *Middleware) remove(address string) {
	m.Lock()
	defer m.Unlock()
	delete(m.conns, address)
}
