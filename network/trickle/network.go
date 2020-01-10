// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package trickle

import (
	"fmt"
	"hash"
	"math/rand"

	"github.com/dchest/siphash"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/identity"
	"github.com/dapperlabs/flow-go/model/trickle"
	"github.com/dapperlabs/flow-go/model/trickle/peer"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/network/trickle/cache"
	"github.com/dapperlabs/flow-go/network/trickle/topology"
	"github.com/dapperlabs/flow-go/protocol"
)

// Network represents the overlay network of our peer-to-peer network, including
// the protocols for handshakes, authentication, gossiping and heartbeats.
type Network struct {
	log     zerolog.Logger
	codec   network.Codec
	state   protocol.State
	me      module.Local
	mw      Middleware
	cache   Cache
	top     Topology
	sip     hash.Hash
	engines map[uint8]network.Engine
}

// NewNetwork creates a new naive overlay network, using the given middleware to
// communicate to direct peers, using the given codec for serialization, and
// using the given state & cache interfaces to track volatile information.
func NewNetwork(log zerolog.Logger, codec network.Codec, state protocol.State, me module.Local, mw Middleware) (*Network, error) {

	top, err := topology.New()
	if err != nil {
		return nil, errors.Wrap(err, "could not initialize topology")
	}

	ca, err := cache.New()
	if err != nil {
		return nil, errors.Wrap(err, "could not initialize cache")
	}

	o := &Network{
		log:     log.With().Str("component", "overlay").Logger(),
		codec:   codec,
		state:   state,
		me:      me,
		mw:      mw,
		top:     top,
		cache:   ca,
		sip:     siphash.New([]byte("daflowtrickleson")),
		engines: make(map[uint8]network.Engine),
	}

	return o, nil
}

// Ready returns a channel that will close when the network stack is ready.
func (n *Network) Ready() <-chan struct{} {
	ready := make(chan struct{})
	n.mw.Start(n)
	go func() {
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
func (n *Network) Register(engineID uint8, engine network.Engine) (network.Conduit, error) {

	// check if the engine engineID is already taken
	_, ok := n.engines[engineID]
	if ok {
		return nil, errors.Errorf("engine already registered (%d)", engine)
	}

	// add the engine ID to the cache
	n.cache.Add(engineID)

	// create the conduit
	conduit := &Conduit{
		engineID: engineID,
		submit:   n.submit,
	}

	// register engine with provided engineID
	n.engines[engineID] = engine

	return conduit, nil
}

// Address implements a callback to provide the overlay layer with the
// addresses we want to establish new connections to.
func (n *Network) Address() (string, error) {

	// get a list of other nodes that are not us, and we are not connected to
	nodeIDs := n.top.Peers().NodeIDs()
	nodeIDs = append(nodeIDs, n.me.NodeID())
	ids, err := n.state.Final().Identities(identity.Not(identity.HasNodeID(nodeIDs...)))
	if err != nil {
		return "", errors.Wrap(err, "could not get identities")
	}

	// if we don't have nodes available, we can't do anything
	if len(ids) == 0 {
		return "", errors.New("no identities available")
	}

	// select a random identity from the list
	id := ids[rand.Int()%len(ids)]

	return id.Address, nil
}

// HandleHandshake implements a callback to validate new connections on the
// overlay layer, allowing us to implement peer authentication on first contact.
func (n *Network) Handshake(conn Connection) (flow.Identifier, error) {

	// initialize our own authentication message
	out := &trickle.Auth{
		NodeID: n.me.NodeID(),
	}
	err := conn.Send(out)
	if err != nil {
		return flow.Identifier{}, errors.Wrap(err, "could not send outgoing auth")
	}

	// read their authentication message
	msg, err := conn.Receive()
	if err != nil {
		return flow.Identifier{}, errors.Wrap(err, "could not receive incoming auth")
	}

	// type assert the response
	in, ok := msg.(*trickle.Auth)
	if !ok {
		return flow.Identifier{}, errors.Errorf("invalid type for incoming auth (%T)", msg)
	}

	// check if this node is ourselves
	if in.NodeID == n.me.NodeID() {
		return flow.Identifier{}, errors.New("connections to self forbidden")
	}

	// check if we are already connected to this peer
	ok = n.top.IsUp(in.NodeID)
	if ok {
		return flow.Identifier{}, errors.Errorf("node already connected (%s)", in.NodeID)
	}

	// check if the committee has a node with the given ID
	_, err = n.state.Final().Identity(in.NodeID)
	if err != nil {
		return flow.Identifier{}, errors.Errorf("unknown nodeID (%s)", in.NodeID)
	}

	// initialize the peer state using the address as its ID
	n.top.Up(in.NodeID)

	return in.NodeID, nil
}

// Cleanup implements a callback to handle peers that have been dropped
// by the middleware layer.
func (n *Network) Cleanup(nodeID flow.Identifier) error {

	// drop the peer state using the ID we registered
	n.top.Down(nodeID)

	return nil
}

// Receive provides a callback to handle the incoming message from the
// given peer.
func (n *Network) Receive(nodeID flow.Identifier, msg interface{}) error {
	var err error
	switch m := msg.(type) {
	case *trickle.Announce:
		err = n.processAnnounce(nodeID, m)
	case *trickle.Request:
		err = n.processRequest(nodeID, m)
	case *trickle.Response:
		err = n.processResponse(nodeID, m)
	default:
		err = errors.Errorf("invalid message type (%T)", m)
	}
	if err != nil {
		return errors.Wrap(err, "could not process message")
	}
	return nil
}

// pack will use the codec to encode an event and return both the payload
// hash and the payload.
func (n *Network) pack(engineID uint8, event interface{}) ([]byte, []byte, error) {

	// encode the payload using the configured codec
	payload, err := n.codec.Encode(event)
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not encode event")
	}

	// use a hash with an engine-specific salt to get the payload hash
	sip := siphash.New([]byte("trickleengine" + fmt.Sprintf("%03d", engineID)))
	return sip.Sum(payload), payload, nil
}

// submit will submit the given event for the given engine to the overlay layer
// for processing; it is used by engines through conduits.
func (n *Network) submit(engineID uint8, event interface{}, targetIDs ...flow.Identifier) error {

	// pack the event to get payload and event ID
	eventID, payload, err := n.pack(engineID, event)
	if err != nil {
		return errors.Wrap(err, "could not pack event")
	}

	err = n.gossip(engineID, eventID, payload, targetIDs...)
	if err != nil {
		return errors.Wrap(err, "could not gossip event")
	}

	return nil
}

// gossip will cache the event and announce it to everyone wha hasn't seen it.
func (n *Network) gossip(engineID uint8, eventID []byte, payload []byte, targetIDs ...flow.Identifier) error {

	// check if the event is already in the cache
	ok := n.cache.Has(engineID, eventID)
	if ok {
		return nil
	}

	// add gossip message to cache
	res := &trickle.Response{
		EngineID:  engineID,
		EventID:   eventID,
		OriginID:  n.me.NodeID(),
		TargetIDs: targetIDs,
		Payload:   payload,
	}
	n.cache.Set(engineID, eventID, res)

	// get all peers that haven't seen it yet
	nodeIDs := n.top.Peers(peer.Not(peer.HasSeen(eventID))).NodeIDs()
	if len(nodeIDs) == 0 {
		return nil
	}

	// announce the event to the selected peers
	ann := &trickle.Announce{
		EngineID: engineID,
		EventID:  eventID,
	}
	err := n.send(ann, nodeIDs...)
	if err != nil {
		return errors.Wrap(err, "could not send announce")
	}

	return nil
}

// processAnnounce will check if we have already seen the announced event and
// request it if it's new for us.
func (n *Network) processAnnounce(nodeID flow.Identifier, ann *trickle.Announce) error {

	// remember that this peer has this event
	n.top.Seen(nodeID, ann.EventID)

	// see if we have already cached this event
	ok := n.cache.Has(ann.EngineID, ann.EventID)
	if ok {
		return nil
	}

	// request the event from the peer
	req := &trickle.Request{
		EngineID: ann.EngineID,
		EventID:  ann.EventID,
	}
	err := n.mw.Send(nodeID, req)
	if err != nil {
		return errors.Wrap(err, "could not send request")
	}

	return nil
}

// processRequest will process the given request and fulfill it.
func (n *Network) processRequest(nodeID flow.Identifier, req *trickle.Request) error {

	// check to find ID and payload in cache
	res, ok := n.cache.Get(req.EngineID, req.EventID)
	if !ok {
		return nil
	}

	// send the response message to the peer
	err := n.mw.Send(nodeID, res)
	if err != nil {
		return errors.Wrap(err, "could not send gossip")
	}

	return nil
}

// processResponse will process the payload of an event sent to us by a peer.
func (n *Network) processResponse(nodeID flow.Identifier, res *trickle.Response) error {

	// mark this event as seen for this peer (usually redundant)
	n.top.Seen(nodeID, res.EventID)

	// check if we already have the event in the cache
	ok := n.cache.Has(res.EngineID, res.EventID)
	if ok {
		return nil
	}

	// check if we can process the event
	err := n.processEvent(res)
	if err != nil {
		return errors.Wrap(err, "could not process event")
	}

	return nil
}

// processEvent will try to process the event contained in a gossip message.
func (n *Network) processEvent(res *trickle.Response) error {

	// if we don't have the engine, don't worry about processing
	engine, ok := n.engines[res.EngineID]
	if !ok {
		return nil
	}

	// if we are not an intended recipient of the message, don't worry about it
	// either
	ok = n.isfor(n.me.NodeID(), res)
	if !ok {
		return nil
	}

	// try to decode the event
	event, err := n.codec.Decode(res.Payload)
	if err != nil {
		return errors.Wrap(err, "could not decode event")
	}

	// try to process the event
	err = engine.Process(res.OriginID, event)
	if err != nil {
		return errors.Wrap(err, "could not process event")
	}

	return nil
}

func (n *Network) send(msg interface{}, nodeIDs ...flow.Identifier) error {

	// send the message through the peer connection
	var result *multierror.Error
	for _, nodeID := range nodeIDs {
		err := n.mw.Send(nodeID, msg)
		if err != nil {
			result = multierror.Append(result, err)
		}
	}

	return result.ErrorOrNil()
}

func (n *Network) isfor(nodeID flow.Identifier, res *trickle.Response) bool {

	// if there are no targets, the message is for broadcasting
	if len(res.TargetIDs) == 0 {
		return true
	}

	// if there are targets, only bubble up if we are one of them
	for _, targetID := range res.TargetIDs {
		if targetID == nodeID {
			return true
		}
	}

	return false
}
