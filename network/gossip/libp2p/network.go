package libp2p

import (
	"context"
	"fmt"
	"sync"

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
	"github.com/dapperlabs/flow-go/network/gossip/libp2p/queue"
)

// Network represents the overlay network of our peer-to-peer network, including
// the protocols for handshakes, authentication, gossiping and heartbeats.
type Network struct {
	sync.RWMutex
	logger  zerolog.Logger
	codec   network.Codec
	ids     flow.IdentityList
	me      module.Local
	mw      middleware.Middleware
	top     middleware.Topology
	metrics module.NetworkMetrics
	engines map[uint8]network.Engine
	rcache  *cache.RcvCache // used to deduplicate incoming messages
	queue   queue.MessageQueue
	cancel  context.CancelFunc
}

// NewNetwork creates a new naive overlay network, using the given middleware to
// communicate to direct peers, using the given codec for serialization, and
// using the given state & cache interfaces to track volatile information.
// csize determines the size of the cache dedicated to keep track of received messages
func NewNetwork(
	log zerolog.Logger,
	codec network.Codec,
	ids flow.IdentityList,
	me module.Local,
	mw middleware.Middleware,
	csize int,
	top middleware.Topology,
	metrics module.NetworkMetrics,
) (*Network, error) {

	rcache, err := cache.NewRcvCache(csize)
	if err != nil {
		return nil, fmt.Errorf("could not initialize cache: %w", err)
	}

	o := &Network{
		logger:  log,
		codec:   codec,
		me:      me,
		mw:      mw,
		engines: make(map[uint8]network.Engine),
		rcache:  rcache,
		top:     top,
		metrics: metrics,
	}

	o.SetIDs(ids)

	ctx, cancel := context.WithCancel(context.Background())
	o.cancel = cancel

	// setup the message queue
	// create priority queue
	o.queue = queue.NewMessageQueue(ctx, queue.GetEventPriority)

	// create workers to read from the queue and call queueSubmitFunc
	queue.CreateQueueWorkers(ctx, queue.DefaultNumWorkers, o.queue, o.queueSubmitFunc)

	return o, nil
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
		n.cancel()
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
		submit:    n.submit,
		publish:   n.publish,
		unicast:   n.unicast,
		multicast: n.multicast,
	}

	// register engine with provided engineID
	n.engines[channelID] = engine
	return conduit, nil
}

// Identity returns a map of all flow.Identifier to flow identity by querying the flow state
func (n *Network) Identity() (map[flow.Identifier]flow.Identity, error) {
	identifierToID := make(map[flow.Identifier]flow.Identity)
	for _, id := range n.ids {
		identifierToID[id.NodeID] = *id
	}
	return identifierToID, nil
}

// Topology returns the identities of a uniform subset of nodes in protocol state using the topology provided earlier
func (n *Network) Topology() (map[flow.Identifier]flow.Identity, error) {
	return n.top.Subset(n.ids, n.fanout(), n.me.NodeID().String())
}

func (n *Network) Receive(nodeID flow.Identifier, msg *message.Message) error {

	err := n.processNetworkMessage(nodeID, msg)
	if err != nil {
		return fmt.Errorf("could not process message: %w", err)
	}

	return nil
}

func (n *Network) SetIDs(ids flow.IdentityList) {
	// remove this node id from the list of fanout target ids to avoid self-dial
	idsMinusMe := ids.Filter(n.me.NotMeFilter())
	n.ids = idsMinusMe
}

// fanout returns the node fanout derived from the identity list
func (n *Network) fanout() int {
	// fanout is currently set to half of the system size for connectivity assurance
	return (len(n.ids) + 1) / 2
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

	// extract channel id
	channelID := uint8(message.ChannelID)

	// Convert message payload to a known message type
	decodedMessage, err := n.codec.Decode(message.Payload)
	if err != nil {
		return fmt.Errorf("could not decode event: %w", err)
	}

	// create queue message
	qm := queue.QueueMessage{
		Payload:   decodedMessage,
		Size:      message.Size(),
		ChannelID: channelID,
		SenderID:  senderID,
	}

	// insert the message in the queue
	err = n.queue.Insert(qm)
	if err != nil {
		return fmt.Errorf("failed to insert message in queue: %w", err)
	}

	return nil
}

// genNetworkMessage uses the codec to encode an event into a NetworkMessage
func (n *Network) genNetworkMessage(channelID uint8, event interface{}, targetIDs ...flow.Identifier) (*message.Message, error) {
	// encode the payload using the configured codec
	payload, err := n.codec.Encode(event)
	if err != nil {
		return nil, fmt.Errorf("could not encode event: %w", err)
	}

	// use a hash with an engine-specific salt to get the payload hash
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

	var emTargets [][]byte
	for _, targetID := range targetIDs {
		tempID := targetID // avoid capturing loop variable
		emTargets = append(emTargets, tempID[:])
	}

	// get origin ID (inplace slicing n.me.NodeID()[:] doesn't work)
	selfID := n.me.NodeID()
	originID := selfID[:]

	// cast event to a libp2p.Message
	msg := &message.Message{
		ChannelID: uint32(channelID),
		EventID:   payloadHash,
		OriginID:  originID,
		TargetIDs: emTargets,
		Payload:   payload,
	}

	return msg, nil
}

// submit method submits the given event for the given channel to the overlay layer
// for processing; it is used by engines through conduits.
func (n *Network) submit(channelID uint8, event interface{}, targetIDs ...flow.Identifier) error {

	// genNetworkMessage the event to get payload and event ID
	msg, err := n.genNetworkMessage(channelID, event, targetIDs...)
	if err != nil {
		return fmt.Errorf("could not cast the event into network message: %w", err)
	}

	// TODO: dedup the message here
	if len(targetIDs) > 1 {
		err = n.mw.Publish(msg, channelID)
	} else if len(targetIDs) == 1 {
		err = n.mw.SendDirect(msg, targetIDs[0])
	} else {
		return fmt.Errorf("empty target ID list for the message")
	}

	if err != nil {
		return fmt.Errorf("could not gossip event: %w", err)
	}

	return nil
}

// publish sends the message in an unreliable way to the given recipients.
// In this context, unreliable means that the message is published over a libp2p pub-sub
// channel and can be read by any node subscribed to that channel.
// The selector could be used to optimize or restrict delivery.
func (n *Network) publish(channelID uint8, message interface{}, selector flow.IdentityFilter) error {
	// excludes this instance of network from list of targeted ids (if any)
	// to avoid self loop on delivering this message.
	selector = filter.And(selector, filter.Not(filter.HasNodeID(n.me.NodeID())))

	// extracts list of recipient identities
	recipients := n.ids.Filter(selector)
	if len(recipients) == 0 {
		return fmt.Errorf("empty target ID list for the message")
	}

	// generates network message (encoding) based on list of recipients
	msg, err := n.genNetworkMessage(channelID, message, recipients.NodeIDs()...)
	if err != nil {
		return fmt.Errorf("publish could not generate network message: %w", err)
	}

	// publishes the message through the channelID, however, the message
	// is only restricted to recipients (if they subscribed to channel ID).
	err = n.mw.Publish(msg, channelID)
	if err != nil {
		return fmt.Errorf("could not publish event: %w", err)
	}

	return nil
}

// unicast sends the message in a reliable way to the given recipient.
// It uses 1-1 direct messaging over the underlying network to deliver the message.
// It returns an error if unicasting fails.
func (n *Network) unicast(channelID uint8, message interface{}, targetID flow.Identifier) error {
	if targetID == n.me.NodeID() {
		n.logger.Debug().Msg("network skips self unicasting")
		return nil
	}

	// generates network message (encoding) based on list of recipients
	msg, err := n.genNetworkMessage(channelID, message, targetID)
	if err != nil {
		return fmt.Errorf("unicast could not generate network message: %w", err)
	}

	err = n.mw.SendDirect(msg, targetID)
	if err != nil {
		return fmt.Errorf("failed to send message to %x: %w", targetID, err)
	}

	n.logger.Debug().
		Msg("message successfully unicasted")

	return nil
}

// multicast unreliably sends the specified event over the channelID to the specified number of recipients selected from
// the specified subset.
// The recipients are selected randomly from the set of identities defined by selectors.
func (n *Network) multicast(channelID uint8, message interface{}, num uint, selector flow.IdentityFilter) error {
	// excludes this instance of network from list of targeted ids (if any)
	// to avoid self loop on delivering this message.
	selector = filter.And(selector, filter.Not(filter.HasNodeID(n.me.NodeID())))

	// NOTE: this is where we can add our own selectors, for example to
	// apply a blacklist or exclude nodes that are in exponential backoff
	// because they were unavailable

	// chooses `num`-many recipients based on selector
	recipients := n.ids.Filter(selector).Sample(num).NodeIDs()
	if len(recipients) < int(num) {
		return fmt.Errorf("could not find recepients based on selector: Requested: %d, Found: %d", num, len(recipients))
	}

	// publishes the message to the recipients
	if err := n.publish(channelID, message, filter.HasNodeID(recipients...)); err != nil {
		return fmt.Errorf("could not multicast message: %w", err)
	}

	n.logger.Debug().
		Msg("message successfully multicasted")

	return nil
}

func (n *Network) queueSubmitFunc(message interface{}) {
	qm := message.(queue.QueueMessage)
	en, found := n.engines[qm.ChannelID]
	if !found {
		n.logger.Error().
			Err(libp2perrors.NewInvalidEngineError(qm.ChannelID, qm.SenderID.String())).
			Msg("failed to submit message")
		return
	}

	// submit the message to the engine synchronously
	err := en.Process(qm.SenderID, qm.Payload)
	if err != nil {
		n.logger.Error().
			Uint8("channel_ID", qm.ChannelID).
			Str("sender_id", qm.SenderID.String()).
			Err(err).
			Msg("failed to process message")
	}
}
