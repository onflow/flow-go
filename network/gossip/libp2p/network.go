package libp2p

import (
	"fmt"
	"sync"

	"github.com/hashicorp/go-multierror"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/crypto/hash"
	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p/cache"
	libp2perrors "github.com/dapperlabs/flow-go/network/gossip/libp2p/errors"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p/message"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p/middleware"
	"github.com/dapperlabs/flow-go/state/protocol"
)

// Network represents the overlay network of our peer-to-peer network, including
// the protocols for handshakes, authentication, gossiping and heartbeats.
type Network struct {
	sync.RWMutex
	logger  zerolog.Logger
	state   protocol.State
	me      module.Local
	codec   network.Codec
	mw      middleware.Middleware
	top     middleware.Topology
	metrics module.NetworkMetrics
	engines map[uint8]network.Engine
	rcache  *cache.RcvCache // used to deduplicate incoming messages
	fanout  int             // used to determine number of nodes' neighbors on overlay
}

// NewNetwork creates a new naive overlay network, using the given middleware to
// communicate to direct peers, using the given codec for serialization, and
// using the given state & cache interfaces to track volatile information.
// csize determines the size of the cache dedicated to keep track of received messages
func NewNetwork(
	log zerolog.Logger,
	state protocol.State,
	me module.Local,
	codec network.Codec,
	mw middleware.Middleware,
	csize int,
	top middleware.Topology,
	metrics module.NetworkMetrics,
) (*Network, error) {

	rcache, err := cache.NewRcvCache(csize)
	if err != nil {
		return nil, fmt.Errorf("could not initialize cache: %w", err)
	}

	// get the list of identities from the protocol state
	identities, err := state.Final().Identities(filter.Any)
	if err != nil {
		return nil, fmt.Errorf("could not query identities: %w", err)
	}

	// fanout is set to half of the system size for connectivity assurance w.h.p
	fanout := (len(identities) + 1) / 2

	n := &Network{
		logger:  log,
		codec:   codec,
		me:      me,
		mw:      mw,
		state:   state,
		engines: make(map[uint8]network.Engine),
		rcache:  rcache,
		fanout:  fanout,
		top:     top,
		metrics: metrics,
	}

	return n, nil
}

// Ready returns a channel that will close when the network stack is ready.
func (n *Network) Ready() <-chan struct{} {
	ready := make(chan struct{})
	go func() {
		err := n.mw.Start(n)
		if err != nil {
			n.logger.Fatal().Err(err).Msg("failed to start middleware")
		}
		close(ready)
	}()
	return ready
}

// Done returns a channel that will close when shutdown is complete.
func (n *Network) Done() <-chan struct{} {
	done := make(chan struct{})
	go func() {
		n.mw.Stop()
		close(done)
	}()
	return done
}

// Register will register the given engine with the given unique engine engineID,
// returning a conduit to directly submit messages to the message bus of the
// engine.
func (n *Network) Register(channelID uint8, engine network.Engine) (network.Conduit, error) {
	n.Lock()
	defer n.Unlock()

	// check if the engine engineID is already taken
	_, ok := n.engines[channelID]
	if ok {
		return nil, fmt.Errorf("engine already registered (%d)", engine)
	}

	// Register the middleware for the channelID topic
	err := n.mw.Subscribe(channelID)
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe to channel %d: %w", channelID, err)
	}

	// create the conduit
	conduit := &Conduit{
		channelID: channelID,
		send:      n.send,
		transmit:  n.transmit,
		publish:   n.publish,
	}

	// register engine with provided engineID
	n.engines[channelID] = engine
	return conduit, nil
}

// Topology returns the identities of a uniform subset of nodes in protocol state using the topology provided earlier
func (n *Network) Topology() (map[flow.Identifier]*flow.Identity, error) {
	identities, err := n.state.Final().Identities(filter.Any)
	if err != nil {
		return nil, fmt.Errorf("could not get identities")
	}
	return n.top.Subset(identities, n.fanout, n.me.NodeID().String())
}

// Identity returns the identity associated with the given node ID.
func (n *Network) Identity(nodeID flow.Identifier) (*flow.Identity, error) {
	identities, err := n.state.Final().Identities(filter.HasNodeID(nodeID))
	if err != nil {
		return nil, fmt.Errorf("could not get identities: %w", err)
	}
	if len(identities) == 0 {
		return nil, fmt.Errorf("identity unknown")
	}
	return identities[0], nil
}

func (n *Network) Receive(nodeID flow.Identifier, msg *message.Message) error {

	err := n.processNetworkMessage(nodeID, msg)
	if err != nil {
		return fmt.Errorf("could not process message: %w", err)
	}

	return nil
}

func (n *Network) processNetworkMessage(senderID flow.Identifier, message *message.Message) error {
	// checks the cache for deduplication and adds the message if not already present
	if n.rcache.Add(message.EventID, message.ChannelID) {
		log := n.logger.With().
			Hex("sender_id", senderID[:]).
			Hex("event_id", message.EventID).
			Logger()

		channelName := engine.ChannelName(uint8(message.ChannelID))

		// drops duplicate message
		log.Debug().
			Str("channel", channelName).
			Msg("dropping message due to duplication")
		n.metrics.NetworkDuplicateMessagesDropped(channelName)
		return nil
	}

	// Extract channel id and find the registered engine
	channelID := uint8(message.ChannelID)
	en, found := n.engines[channelID]
	if !found {
		return libp2perrors.NewInvalidEngineError(channelID, senderID.String())
	}

	// Convert message payload to a known message type
	decodedMessage, err := n.codec.Decode(message.Payload)
	if err != nil {
		return fmt.Errorf("could not decode event: %w", err)
	}

	// call the engine asynchronously with the message payload
	en.Submit(senderID, decodedMessage)

	return nil
}

// encodeMessage uses the codec to encode an event into a NetworkMessage
func (n *Network) encodeMessage(channelID uint8, event interface{}) (*message.Message, error) {
	// encode the payload using the configured codec
	payload, err := n.codec.Encode(event)
	if err != nil {
		return nil, fmt.Errorf("could not encode event: %w", err)
	}

	// use a hash with a channel-specific salt to get the payload hash
	h := hash.NewSHA3_384()
	_, err = h.Write([]byte("libp2ppacking" + fmt.Sprintf("%03d", channelID)))
	if err != nil {
		return nil, fmt.Errorf("could not hash channel ID as salt: %w", err)
	}

	_, err = h.Write(payload)
	if err != nil {
		return nil, fmt.Errorf("could not hash event: %w", err)
	}

	payloadHash := h.SumHash()

	// get origin ID (inplace slicing n.me.NodeID()[:] doesn't work)
	selfID := n.me.NodeID()
	originID := selfID[:]

	// cast event to a libp2p.Message
	msg := &message.Message{
		ChannelID: uint32(channelID),
		EventID:   payloadHash,
		OriginID:  originID,
		Payload:   payload,
	}

	return msg, nil
}

// transmit sends the message in a reliable way to the given recipients. It will error if it fails
// to deliver the message to one of the recipients.
func (n *Network) transmit(channelID uint8, message interface{}, recipientIDs ...flow.Identifier) error {

	// try to encode the payload first
	msg, err := n.encodeMessage(channelID, message)
	if err != nil {
		return fmt.Errorf("could not encode message: %w", err)
	}

	var result *multierror.Error
	for _, recipientID := range recipientIDs {
		if recipientID == n.me.NodeID() {
			n.logger.Debug().Msg("ignoring transmit to self")
			continue
		}
		err = n.mw.Send(msg, recipientID)
		if err != nil {
			result = multierror.Append(result, err)
			continue
		}
	}

	return result.ErrorOrNil()
}

// send sends the message in a reliable way to the given number of recipients, selected randomly from
// the set of identities selected by the selectors. In this context, reliable means that the message
// is sent across the network over a one-to-one connection.
func (n *Network) send(channelID uint8, message interface{}, num uint, selector flow.IdentityFilter) error {

	// we always exclude ourselves
	selector = filter.And(selector, filter.Not(filter.HasNodeID(n.me.NodeID())))

	// try to encode the payload first
	msg, err := n.encodeMessage(channelID, message)
	if err != nil {
		return fmt.Errorf("could not encode message: %w", err)
	}

	// NOTE: this is where we can add our own selectors, for example to
	// apply a blacklist or exclude nodes that are in exponential backoff
	// because they were unavailable

	// get the set of identities from the protocol state using the selector
	recipients, err := n.state.Final().Identities(selector)
	if err != nil {
		return fmt.Errorf("could not get identities: %w", err)
	}

	// execute the delivery function to select a recipient until done
	sent := uint(0)
	for sent < num {

		// if there are no valid recipients left, bail
		if len(recipients) == 0 {
			return fmt.Errorf("no valid recipients available (%d of %d sent)", num, sent)
		}

		// select a random recipient and remove from valid set of recipients
		recipientID := recipients.Sample(1)[0].NodeID
		recipients = recipients.Filter(filter.Not(filter.HasNodeID(recipientID)))

		// send the message and log if we skip due to error
		msg.TargetIDs = [][]byte{recipientID[:]}
		err = n.mw.Send(msg, recipientID)
		if err != nil {
			// TODO: mark as unavailable, start exponential backoff
			n.logger.Debug().Hex("recipient", recipientID[:]).Msg("skipping unavailable recipient")
			continue
		}

		// increase the sent count
		sent++
	}

	return nil
}

// publish sends the message in an unreliable way to the given recipients. In this context, unreliable
// means that the message is published over a libp2p pub-sub channel and can be read by any node
// subscribed to that channel. The selector could be used to optimize or restrict delivery.
func (n *Network) publish(channelID uint8, message interface{}, selector flow.IdentityFilter) error {

	// we always exclude ourselves
	selector = filter.And(selector, filter.Not(filter.HasNodeID(n.me.NodeID())))

	// first, we try to encode the network message
	msg, err := n.encodeMessage(channelID, message)
	if err != nil {
		return fmt.Errorf("could not encode message: %w", err)
	}

	// get the set of identities from the protocol state using the selector
	// NOTE: we currently send to everyone regardless, so we could just
	// remove this by changing the networking implementation a bit
	recipients, err := n.state.Final().Identities(selector)
	if err != nil {
		return fmt.Errorf("could not get identities: %w", err)
	}

	// set the target list and publish the message
	targetIDs := make([][]byte, 0, len(recipients))
	for _, recipient := range recipients {
		targetIDs = append(targetIDs, recipient.NodeID[:])
	}
	err = n.mw.Publish(channelID, msg)
	if err != nil {
		return fmt.Errorf("could not publish event")
	}

	return nil
}
