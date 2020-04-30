// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package libp2p

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strconv"
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

// the inbound message queue size for One to One and One to K messages (each)
const InboundMessageQueueSize = 1000

// Middleware handles the input & output on the direct connections we have to
// our neighbours on the peer-to-peer network.
type Middleware struct {
	sync.Mutex
	ctx        context.Context
	cancel     context.CancelFunc
	log        zerolog.Logger
	codec      network.Codec
	ov         middleware.Overlay
	wg         *sync.WaitGroup
	libP2PNode *P2PNode
	stop       chan struct{}
	me         flow.Identifier
	host       string
	port       string
	key        crypto.PrivateKey
	metrics    module.Metrics
	validators []validators.MessageValidator
}

// NewMiddleware creates a new middleware instance with the given config and using the
// given codec to encode/decode messages to our peers.
func NewMiddleware(log zerolog.Logger, codec network.Codec, address string, flowID flow.Identifier,
	key crypto.PrivateKey, metrics module.Metrics, validators ...validators.MessageValidator) (*Middleware, error) {
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

	// create the node entity and inject dependencies & config
	m := &Middleware{
		ctx:        ctx,
		cancel:     cancel,
		log:        log,
		codec:      codec,
		libP2PNode: p2p,
		wg:         &sync.WaitGroup{},
		stop:       make(chan struct{}),
		me:         flowID,
		host:       ip,
		port:       port,
		key:        key,
		metrics:    metrics,
		validators: validators,
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

	// create PubSub options for libp2p to use the discovery object
	psOptions := []pubsub.Option{pubsub.WithDiscovery(d),
		pubsub.WithMessageSigning(false),
		pubsub.WithStrictSignatureVerification(false),
	}

	nodeAddress := NodeAddress{Name: m.me.String(), IP: m.host, Port: m.port}

	libp2pKey, err := PrivKey(m.key)
	if err != nil {
		return fmt.Errorf("failed to translate Flow key to Libp2p key: %w", err)
	}

	// start the libp2p node
	err = m.libP2PNode.Start(m.ctx, nodeAddress, m.log, libp2pKey, m.handleIncomingStream, psOptions...)

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

	// cancel the context (this also signals libp2p go routines to exit)
	m.cancel()

	// stop libp2p
	done, err := m.libP2PNode.Stop()
	if err != nil {
		m.log.Error().Err(err).Msg("stopping failed")
	} else {
		<-done
		m.log.Debug().Msg("node stopped successfully")
	}

	// wait for the go routines spawned by middleware to stop
	m.wg.Wait()
}

// Send sends the message to the set of target ids
// If there is only one target NodeID, then a direct 1-1 connection is used by calling middleware.sendDirect
// Otherwise, middleware.publish is used, which uses the PubSub method of communication.
func (m *Middleware) Send(channelID uint8, msg *message.Message, targetIDs ...flow.Identifier) error {
	var err error
	mode := m.chooseMode(channelID, msg, targetIDs...)
	// decide what mode of communication to use
	switch mode {
	case NoOp:
		// NOTE: we can't error on this at the moment, because single nodes of
		// a role in tests will attempt to send messages like this, which should
		// be a no-op, but not an error
		m.log.Debug().Msg("send to no-one")
		return nil
	case OneToOne:
		if targetIDs[0] == m.me {
			// to avoid self dial by the underlay
			m.log.Debug().Msg("send to self")
			return nil
		}
		err = m.sendDirect(targetIDs[0], msg)
	case OneToK:
		err = m.publish(channelID, msg)
	default:
		err = fmt.Errorf("invalid communcation mode: %d", mode)
	}

	if err != nil {
		return fmt.Errorf("failed to send message to %s:%w", targetIDs, err)
	}
	return nil
}

// chooseMode determines the communication mode to use. Currently it only considers the length of the targetIDs.
func (m *Middleware) chooseMode(_ uint8, _ *message.Message, targetIDs ...flow.Identifier) communicationMode {
	switch len(targetIDs) {
	case 0:
		return NoOp
	case 1:
		return OneToOne
	default:
		return OneToK
	}
}

// sendDirect will try to send the given message to the given peer utilizing a 1-1 direct connection
func (m *Middleware) sendDirect(targetID flow.Identifier, msg *message.Message) error {

	// get an identity to connect to. The identity provides the destination TCP address.
	idsMap, err := m.ov.Identity()
	if err != nil {
		return fmt.Errorf("could not get identities: %w", err)
	}
	flowIdentity, found := idsMap[targetID]
	if !found {
		return fmt.Errorf("could not get identity for %s: %w", targetID.String(), err)
	}

	// create new stream
	// (streams don't need to be reused and are fairly inexpensive to be created for each send.
	// A stream creation does NOT incur an RTT as stream negotiation happens as part of the first message
	// sent out the the receiver
	stream, err := m.connect(flowIdentity.NodeID.String(), flowIdentity.Address, flowIdentity.NetworkPubKey)
	if err != nil {
		return fmt.Errorf("could not create new stream for %s: %w", targetID.String(), err)
	}

	// create a gogo protobuf writer
	bufw := bufio.NewWriter(stream)
	writer := ggio.NewDelimitedWriter(bufw)

	err = writer.WriteMsg(msg)
	if err != nil {
		return fmt.Errorf("failed to send message to %s: %w", targetID.String(), err)
	}

	// flush the stream
	err = bufw.Flush()
	if err != nil {
		return fmt.Errorf("failed to flush stream for %s: %w", targetID.String(), err)
	}

	// track the number of bytes that will be written to the wire for metrics
	byteCount := bufw.Buffered()

	// flush the stream
	err = bufw.Flush()
	if err != nil {
		return fmt.Errorf("failed to flush stream for %s: %w", targetID.String(), err)
	}

	// close the stream immediately
	go helpers.FullClose(stream)

	// OneToOne communication metrics are reported with topic OneToOne
	go m.reportOutboundMsgSize(byteCount, metrics.TopicLabelOneToOne)

	return nil
}

// connect creates a new stream
func (m *Middleware) connect(flowID string, address string, key crypto.PublicKey) (libp2pnetwork.Stream, error) {

	ip, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("could not parse address %s:%v", address, err)
	}

	// convert the Flow key to a LibP2P key
	lkey, err := PublicKey(key)
	if err != nil {
		return nil, fmt.Errorf("could not convert flow key to libp2p key: %v", err)
	}

	// Create a new NodeAddress
	nodeAddress := NodeAddress{Name: flowID, IP: ip, Port: port, PubKey: lkey}

	// Create a stream for it
	stream, err := m.libP2PNode.CreateStream(m.ctx, nodeAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to create stream for %s:%v", nodeAddress.Name, err)
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
		case <-conn.done:
			m.log.Info().Msg("exiting process loop: connection stops")
			break ProcessLoop
		case msg, ok := <-conn.inbound:
			if !ok {
				m.log.Info().Msg("exiting process loop: connection stops")
				break ProcessLoop
			}

			msgSize := msg.Size()
			m.reportInboundMsgSize(msgSize, metrics.TopicLabelOneToOne)

			m.processMessage(msg)
			continue ProcessLoop
		}
	}

	log.Info().Msg("middleware closed the connection")
}

// Subscribe will subscribe the middleware for a topic with the same name as the channelID
func (m *Middleware) Subscribe(channelID uint8) error {

	// a Flow ChannelID becomes the topic ID in libp2p
	topic := topicFromChannelID(channelID)

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
		case msg, ok := <-rs.inbound:
			if !ok {
				m.log.Info().Msg("exiting subscription loop: connection stops")
				break SubscriptionLoop
			}

			channelID, err := channelIDFromTopic(rs.sub.Topic())
			if err != nil {
				m.log.Error().Err(err).Str("channel_id", channelID).Msg("failed to report metric")
			} else {
				msgSize := msg.Size()
				m.reportInboundMsgSize(msgSize, channelID)
			}

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
func (m *Middleware) publish(channelID uint8, msg *message.Message) error {

	// convert the message to bytes to be put on the wire.
	data, err := msg.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal the message: %w", err)
	}

	topic := topicFromChannelID(channelID)

	// publish the bytes on the topic
	err = m.libP2PNode.Publish(m.ctx, topic, data)
	if err != nil {
		return fmt.Errorf("failed to publish the message: %w", err)
	}

	go m.reportOutboundMsgSize(len(data), engine.ChannelName(channelID))

	return nil
}

func (m *Middleware) reportOutboundMsgSize(size int, topic string) {
	m.metrics.NetworkMessageSent(size, topic)
}

func (m *Middleware) reportInboundMsgSize(size int, topic string) {
	m.metrics.NetworkMessageReceived(size, topic)
}

// topicFromChannelID converts a Flow channel ID to LibP2P pub-sub topic id e.g. 10 -> "10"
func topicFromChannelID(channelID uint8) string {
	return strconv.Itoa(int(channelID))
}

// channelIDFromTopic converts a LibP2P pub-sub topic id to a channel ID string e.g. "10" -> "CollectionProvider"
func channelIDFromTopic(topic string) (string, error) {
	channelID, err := strconv.ParseUint(topic, 10, 8)
	if err != nil {
		return "", fmt.Errorf(" failed to get channeld ID from topic: %w", err)
	}
	return engine.ChannelName(uint8(channelID)), nil
}
