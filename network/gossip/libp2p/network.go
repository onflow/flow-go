package libp2p

import (
	"context"
	"fmt"
	"strings"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/gossip/libp2p/cache"
	"github.com/onflow/flow-go/network/gossip/libp2p/message"
	"github.com/onflow/flow-go/network/gossip/libp2p/middleware"
	"github.com/onflow/flow-go/network/gossip/libp2p/queue"
	"github.com/onflow/flow-go/network/gossip/libp2p/topology"
)

type identifierFilter func(ids ...flow.Identifier) ([]flow.Identifier, error)

// Network represents the overlay network of our peer-to-peer network, including
// the protocols for handshakes, authentication, gossiping and heartbeats.
type Network struct {
	logger          zerolog.Logger
	codec           network.Codec
	ids             flow.IdentityList
	me              module.Local
	role            flow.Role
	mw              middleware.Middleware
	top             topology.Topology
	metrics         module.NetworkMetrics
	rcache          *cache.RcvCache // used to deduplicate incoming messages
	queue           queue.MessageQueue
	ctx             context.Context
	cancel          context.CancelFunc
	subscriptionMgr *subscriptionManager
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
	role flow.Role,
	mw middleware.Middleware,
	csize int,
	top topology.Topology,
	metrics module.NetworkMetrics,
) (*Network, error) {

	rcache, err := cache.NewRcvCache(csize)
	if err != nil {
		return nil, fmt.Errorf("could not initialize cache: %w", err)
	}

	o := &Network{
		logger:          log,
		codec:           codec,
		me:              me,
		role:            role,
		mw:              mw,
		rcache:          rcache,
		top:             top,
		metrics:         metrics,
		subscriptionMgr: newSubscriptionManager(mw),
	}

	o.SetIDs(ids)

	ctx, cancel := context.WithCancel(context.Background())
	o.cancel = cancel
	o.ctx = ctx

	// setup the message queue
	// create priority queue
	o.queue = queue.NewMessageQueue(ctx, queue.GetEventPriority, metrics)

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
func (n *Network) Register(channelID string, engine network.Engine) (network.Conduit, error) {

	err := n.subscriptionMgr.register(channelID, engine)
	if err != nil {
		return nil, fmt.Errorf("failed to register engine for channel %s: %w", channelID, err)
	}

	// create a cancellable child context
	ctx, cancel := context.WithCancel(n.ctx)

	// create the conduit
	conduit := &Conduit{
		ctx:       ctx,
		cancel:    cancel,
		channelID: channelID,
		submit:    n.submit,
		publish:   n.publish,
		unicast:   n.unicast,
		multicast: n.multicast,
		close:     n.unregister,
	}

	return conduit, nil
}

// unregister unregisters the engine for the specified channel. The engine will no longer be able to send or
// receive messages from that channelID
func (n *Network) unregister(channelID string) error {
	err := n.subscriptionMgr.unregister(channelID)
	if err != nil {
		return fmt.Errorf("failed to unregister engine for channelID %s: %w", channelID, err)
	}
	return nil
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
	myTopics := engine.GetTopicsByRole(n.role)
	var myFanout flow.IdentityList

	// samples a connected component fanout from each topic and takes the
	// union of all fanouts.
	for _, topic := range myTopics {
		subset, err := n.top.Subset(n.ids, n.fanout(), topic)
		if err != nil {
			return nil, fmt.Errorf("failed to derive list of peer nodes to connect for topic %s: %w", topic, err)
		}
		myFanout = append(myFanout, subset.Filter(filter.Not(filter.In(myFanout)))...)
	}

	// creates a map of all the selected ids
	topMap := make(map[flow.Identifier]flow.Identity)
	for _, id := range myFanout {
		topMap[id.NodeID] = *id
	}
	return topMap, nil
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
func (n *Network) fanout() uint {
	// fanout is currently set to half of the system size for connectivity assurance
	return uint(len(n.ids)+1) / 2
}

func (n *Network) processNetworkMessage(senderID flow.Identifier, message *message.Message) error {
	// checks the cache for deduplication and adds the message if not already present
	if n.rcache.Add(message.EventID, message.ChannelID) {
		log := n.logger.With().
			Hex("sender_id", senderID[:]).
			Hex("event_id", message.EventID).
			Logger()

		// drops duplicate message
		log.Debug().
			Str("channel", message.ChannelID).
			Msg("dropping message due to duplication")

		n.metrics.NetworkDuplicateMessagesDropped(message.ChannelID, message.Type)

		return nil
	}

	// Convert message payload to a known message type
	decodedMessage, err := n.codec.Decode(message.Payload)
	if err != nil {
		return fmt.Errorf("could not decode event: %w", err)
	}

	// create queue message
	qm := queue.QueueMessage{
		Payload:   decodedMessage,
		Size:      message.Size(),
		ChannelID: message.ChannelID,
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
func (n *Network) genNetworkMessage(channelID string, event interface{}, targetIDs ...flow.Identifier) (*message.Message, error) {
	// encode the payload using the configured codec
	payload, err := n.codec.Encode(event)
	if err != nil {
		return nil, fmt.Errorf("could not encode event: %w", err)
	}

	// use a hash with an engine-specific salt to get the payload hash
	h := hash.NewSHA3_384()
	_, err = h.Write([]byte("libp2ppacking" + channelID))
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

	// get origin ID
	selfID := n.me.NodeID()
	originID := selfID[:]

	// get message type from event type and remove the asterisk prefix if present
	msgType := strings.TrimLeft(fmt.Sprintf("%T", event), "*")

	// cast event to a libp2p.Message
	msg := &message.Message{
		ChannelID: channelID,
		EventID:   payloadHash,
		OriginID:  originID,
		TargetIDs: emTargets,
		Payload:   payload,
		Type:      msgType,
	}

	return msg, nil
}

// submit method submits the given event for the given channel to the overlay layer
// for processing; it is used by engines through conduits.
func (n *Network) submit(channelID string, event interface{}, targetIDs ...flow.Identifier) error {

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

// unicast sends the message in a reliable way to the given recipient.
// It uses 1-1 direct messaging over the underlying network to deliver the message.
// It returns an error if unicasting fails.
func (n *Network) unicast(channelID string, message interface{}, targetID flow.Identifier) error {
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

// publish sends the message in an unreliable way to the given recipients.
// In this context, unreliable means that the message is published over a libp2p pub-sub
// channel and can be read by any node subscribed to that channel.
// The selector could be used to optimize or restrict delivery.
func (n *Network) publish(channelID string, message interface{}, targetIDs ...flow.Identifier) error {

	err := n.sendOnChannel(channelID, message, targetIDs, n.removeSelfFilter)

	if err != nil {
		return fmt.Errorf("failed to publish on channel ID %s: %w", channelID, err)
	}

	n.logger.
		Debug().
		Str("channel_id", channelID).
		Msg("message successfully published")

	return nil
}

// multicast unreliably sends the specified event over the channelID to randomly selected 'num' number of recipients
// selected from the specified targetIDs.
func (n *Network) multicast(channelID string, message interface{}, num uint, targetIDs ...flow.Identifier) error {

	filters := []identifierFilter{n.removeSelfFilter, sampleFilter(num)}

	err := n.sendOnChannel(channelID, message, targetIDs, filters...)

	// publishes the message to the selected targets
	if err != nil {
		return fmt.Errorf("failed to multicast on channel ID %s: %w", channelID, err)
	}

	n.logger.
		Debug().
		Str("channel_id", channelID).
		Uint("target_count", num).
		Msg("message successfully multicasted")

	return nil
}

// removeSelfFilter removes the flow.Identifier of this node if present, from the list of nodes
func (n *Network) removeSelfFilter(ids ...flow.Identifier) ([]flow.Identifier, error) {
	targetIDMinusSelf := make([]flow.Identifier, 0, len(ids))
	for _, t := range ids {
		if t != n.me.NodeID() {
			targetIDMinusSelf = append(targetIDMinusSelf, t)
		}
	}
	return targetIDMinusSelf, nil
}

// sampleFilter returns an identitiferFilter which returns a random sample from ids
func sampleFilter(size uint) identifierFilter {
	return func(ids ...flow.Identifier) ([]flow.Identifier, error) {
		return flow.Sample(size, ids...), nil
	}
}

// sendOnChannel sends the message on channelID to targetIDs after applying the all the filters to targetIDs
func (n *Network) sendOnChannel(channelID string, message interface{}, targetIDs []flow.Identifier, filters ...identifierFilter) error {

	var err error
	// filter the targetIDs
	for _, f := range filters {
		targetIDs, err = f(targetIDs...)
		// if filter failed
		if err != nil {
			return err
		}
		// if the filtration resulted in an empty list
		if len(targetIDs) == 0 {
			return fmt.Errorf("empty list of target ids")
		}
	}

	// generate network message (encoding) based on list of recipients
	msg, err := n.genNetworkMessage(channelID, message, targetIDs...)
	if err != nil {
		return fmt.Errorf("failed to generate network message for channel ID %s: %w", channelID, err)
	}

	// publish the message through the channelID, however, the message
	// is only restricted to targetIDs (if they subscribed to channel ID).
	err = n.mw.Publish(msg, channelID)
	if err != nil {
		return fmt.Errorf("failed to send message on channel ID %s: %w", channelID, err)
	}

	return nil
}

// queueSubmitFunc submits the message to the engine synchronously. It is the callback for the queue worker
// when it gets a message from the queue
func (n *Network) queueSubmitFunc(message interface{}) {
	qm := message.(queue.QueueMessage)
	eng, err := n.subscriptionMgr.getEngine(qm.ChannelID)
	if err != nil {
		n.logger.Error().
			Err(err).
			Str("channel_id", qm.ChannelID).
			Str("sender_id", qm.SenderID.String()).
			Msg("failed to submit message")
		return
	}

	// submit the message to the engine synchronously
	err = eng.Process(qm.SenderID, qm.Payload)
	if err != nil {
		n.logger.Error().
			Err(err).
			Str("channel_id", qm.ChannelID).
			Str("sender_id", qm.SenderID.String()).
			Msg("failed to process message")
	}
}
