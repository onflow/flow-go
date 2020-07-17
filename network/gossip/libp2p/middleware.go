// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package libp2p

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"sync"

	ggio "github.com/gogo/protobuf/io"
	"github.com/libp2p/go-libp2p-core/helpers"
	libp2pnetwork "github.com/libp2p/go-libp2p-core/network"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p/message"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p/middleware"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p/validators"
)

type communicationMode int

const (
	NoOp communicationMode = iota
	OneToOne
	OneToK
)

const DefaultMaxPubSubMsgSize = 1 << 21 //2mb

// the inbound message queue size for One to One and One to K messages (each)
const InboundMessageQueueSize = 100

// Middleware handles the input & output on the direct connections we have to
// our neighbours on the peer-to-peer network.
type Middleware struct {
	sync.Mutex
	ctx              context.Context
	cancel           context.CancelFunc
	log              zerolog.Logger
	codec            network.Codec
	ov               middleware.Overlay
	wg               *sync.WaitGroup
	libP2PNode       *P2PNode
	stop             chan struct{}
	me               flow.Identifier
	host             string
	port             string
	key              crypto.PrivateKey
	metrics          module.NetworkMetrics
	maxPubSubMsgSize int
	rootBlockID      string
	validators       []validators.MessageValidator
}

// NewMiddleware creates a new middleware instance with the given config and using the
// given codec to encode/decode messages to our peers.
func NewMiddleware(log zerolog.Logger, codec network.Codec, address string, flowID flow.Identifier,
	key crypto.PrivateKey, metrics module.NetworkMetrics, maxPubSubMsgSize int,
	rootBlockID string, validators ...validators.MessageValidator) (*Middleware, error) {
	ip, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}

	p2p := &P2PNode{}
	ctx, cancel := context.WithCancel(context.Background())

	if len(validators) == 0 {
		// add default validators to filter out unwanted messages received by this node
		validators = defaultValidators(log, flowID)
	}

	if maxPubSubMsgSize <= 0 {
		maxPubSubMsgSize = DefaultMaxPubSubMsgSize
	}

	// create the node entity and inject dependencies & config
	m := &Middleware{
		ctx:              ctx,
		cancel:           cancel,
		log:              log,
		codec:            codec,
		libP2PNode:       p2p,
		wg:               &sync.WaitGroup{},
		stop:             make(chan struct{}),
		me:               flowID,
		host:             ip,
		port:             port,
		key:              key,
		metrics:          metrics,
		maxPubSubMsgSize: maxPubSubMsgSize,
		rootBlockID:      rootBlockID,
		validators:       validators,
	}

	return m, err
}

func defaultValidators(log zerolog.Logger, flowID flow.Identifier) []validators.MessageValidator {
	return []validators.MessageValidator{
		validators.NewSenderValidator(flowID),      // validator to filter out messages sent by this node itself
		validators.NewTargetValidator(log, flowID), // validator to filter out messages not intended for this node
	}
}

// Me returns the flow identifier of the this middleware
func (m *Middleware) Me() flow.Identifier {
	return m.me
}

// GetIPPort returns the ip address and port number associated with the middleware
func (m *Middleware) GetIPPort() (string, string) {
	return m.libP2PNode.GetIPPort()
}

func (m *Middleware) PublicKey() crypto.PublicKey {
	return m.key.PublicKey()
}

// Start will start the middleware.
func (m *Middleware) Start(ov middleware.Overlay) error {

	m.ov = ov

	// create a discovery object to help libp2p discover peers
	d := NewDiscovery(m.log, m.ov, m.me, m.stop)

	// create PubSub options for libp2p to use
	psOptions := []pubsub.Option{
		// set the discovery object
		pubsub.WithDiscovery(d),
		// skip message signing
		pubsub.WithMessageSigning(false),
		// skip message signature
		pubsub.WithStrictSignatureVerification(false),
		// set max message size limit for 1-k PubSub messaging
		pubsub.WithMaxMessageSize(m.maxPubSubMsgSize),
	}

	nodeAddress := NodeAddress{Name: m.me.String(), IP: m.host, Port: m.port}

	libp2pKey, err := PrivKey(m.key)
	if err != nil {
		return fmt.Errorf("failed to translate Flow key to Libp2p key: %w", err)
	}

	// start the libp2p node
	err = m.libP2PNode.Start(m.ctx, nodeAddress, m.log, libp2pKey, m.handleIncomingStream, m.rootBlockID, psOptions...)
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

	// stop libp2p
	done, err := m.libP2PNode.Stop()
	if err != nil {
		m.log.Error().Err(err).Msg("stopping failed")
	} else {
		<-done
		m.log.Debug().Msg("node stopped successfully")
	}

	// cancel the context (this also signals any lingering libp2p go routines to exit)
	m.cancel()

	// wait for the go routines spawned by middleware to stop
	m.wg.Wait()
}

// Send will try to send the given message to the given peer utilizing a 1-1 direct connection
func (m *Middleware) Send(msg *message.Message, recipientID flow.Identifier) error {

	// get an identity to connect to. The identity provides the destination TCP address.
	identity, err := m.ov.Identity(recipientID)
	if err != nil {
		return fmt.Errorf("could not get identity: %w", err)
	}

	// create new stream
	// (streams don't need to be reused and are fairly inexpensive to be created for each send.
	// A stream creation does NOT incur an RTT as stream negotiation happens as part of the first message
	// sent out the the receiver
	stream, err := m.connect(identity.NodeID.String(), identity.Address, identity.NetworkPubKey)
	if err != nil {
		return fmt.Errorf("could not create new stream for %s: %w", recipientID.String(), err)
	}

	// create a gogo protobuf writer
	bufw := bufio.NewWriter(stream)
	writer := ggio.NewDelimitedWriter(bufw)

	err = writer.WriteMsg(msg)
	if err != nil {
		return fmt.Errorf("failed to send message to %s: %w", recipientID.String(), err)
	}

	// flush the stream
	err = bufw.Flush()
	if err != nil {
		return fmt.Errorf("failed to flush stream for %s: %w", recipientID.String(), err)
	}

	// track the number of bytes that will be written to the wire for metrics
	byteCount := bufw.Buffered()

	// flush the stream
	err = bufw.Flush()
	if err != nil {
		return fmt.Errorf("failed to flush stream for %s: %w", recipientID.String(), err)
	}

	// close the stream immediately
	go helpers.FullClose(stream)

	// OneToOne communication metrics are reported with topic OneToOne
	go m.reportOutboundMsgSize(byteCount, metrics.ChannelOneToOne)

	return nil
}

// connect creates a new stream
func (m *Middleware) connect(flowID string, address string, key crypto.PublicKey) (libp2pnetwork.Stream, error) {

	ip, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("could not parse address %s: %w", address, err)
	}

	// convert the Flow key to a LibP2P key
	lkey, err := PublicKey(key)
	if err != nil {
		return nil, fmt.Errorf("could not convert flow key to libp2p key: %w", err)
	}

	// Create a new NodeAddress
	nodeAddress := NodeAddress{Name: flowID, IP: ip, Port: port, PubKey: lkey}

	// Create a stream for it
	stream, err := m.libP2PNode.CreateStream(m.ctx, nodeAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to create stream for %s :%w", nodeAddress.Name, err)
	}

	m.log.Info().Str("target_id", flowID).Str("address", address).Msg("stream created")
	return stream, nil
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
		case msg, ok := <-conn.inbound:
			if !ok {
				m.log.Info().Msg("exiting process loop: read connection closed")
				break ProcessLoop
			}

			msgSize := msg.Size()
			m.reportInboundMsgSize(msgSize, metrics.ChannelOneToOne)
			m.processMessage(msg)
			continue ProcessLoop
		}
	}

	log.Info().Msg("middleware closed the connection")
}

// Subscribe will subscribe the middleware for a topic with the fully qualified channel ID name
func (m *Middleware) Subscribe(channelID uint8) error {

	topic := engine.FullyQualifiedChannelName(channelID, m.rootBlockID)

	s, err := m.libP2PNode.Subscribe(m.ctx, topic)
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
	defer rs.stop()
	// process incoming messages for as long as the peer is running
SubscriptionLoop:
	for {
		select {
		case <-m.stop:
			// middleware stops
			m.log.Info().Msg("exiting subscription loop: middleware stops")
			break SubscriptionLoop
		case msg, ok := <-rs.inbound:
			if !ok {
				m.log.Info().Msg("exiting subscription loop: connection stops")
				break SubscriptionLoop
			}

			msgSize := msg.Size()
			m.reportInboundMsgSize(msgSize, rs.sub.Topic())

			m.processMessage(msg)

			continue SubscriptionLoop
		}
	}
}

// processMessage processes a message and eventually passes it to the overlay
func (m *Middleware) processMessage(msg *message.Message) {

	// run through all the message validators
	for _, v := range m.validators {
		// if any one fails, stop message propagation
		if !v.Validate(*msg) {
			return
		}
	}

	// if validation passed, send the message to the overlay
	err := m.ov.Receive(flow.HashToID(msg.OriginID), msg)
	if err != nil {
		m.log.Error().Err(err).Msg("could not deliver payload")
	}
}

// Publish publishes the given payload on the topic
func (m *Middleware) Publish(channelID uint8, msg *message.Message) error {

	// convert the message to bytes to be put on the wire.
	data, err := msg.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal the message: %w", err)
	}

	msgSize := len(data)
	if msgSize > m.maxPubSubMsgSize {
		// libp2p pubsub will silently drop the message if its size is greater than the configured pubsub max message size
		// hence return an error as this message is undeliverable
		return fmt.Errorf("message size %d exceeds configured max message size %d", msgSize, m.maxPubSubMsgSize)
	}

	topic := engine.FullyQualifiedChannelName(channelID, m.rootBlockID)

	// publish the bytes on the topic
	err = m.libP2PNode.Publish(m.ctx, topic, data)
	if err != nil {
		return fmt.Errorf("failed to publish the message: %w", err)
	}

	m.reportOutboundMsgSize(len(data), engine.ChannelName(channelID)) // use the shorter channel name to report metrics

	return nil
}

func (m *Middleware) reportOutboundMsgSize(size int, channel string) {
	m.metrics.NetworkMessageSent(size, channel)
}

func (m *Middleware) reportInboundMsgSize(size int, channel string) {
	m.metrics.NetworkMessageReceived(size, channel)
}
