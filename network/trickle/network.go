// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package trickle

import (
	"fmt"
	"hash"
	"math/rand"
	"time"

	"github.com/dchest/siphash"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/model/filter"
	"github.com/dapperlabs/flow-go/model/trickle"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/network"
)

// Network represents the overlay network of our peer-to-peer network, including
// the protocols for handshakes, authentication, gossiping and heartbeats.
type Network struct {
	log     zerolog.Logger
	codec   network.Codec
	com     module.Committee
	mw      Middleware
	cache   Cache
	state   State
	sip     hash.Hash
	engines map[uint8]network.Engine
}

// NewNetwork creates a new naive overlay network, using the given middleware to
// communicate to direct peers, using the given codec for serialization, and
// using the given state & cache interfaces to track volatile information.
func NewNetwork(log zerolog.Logger, codec network.Codec, com module.Committee, mw Middleware, state State, cache Cache) (*Network, error) {

	o := &Network{
		log:     log,
		codec:   codec,
		com:     com,
		mw:      mw,
		state:   state,
		cache:   cache,
		sip:     siphash.New([]byte("tricklehashseedx")),
		engines: make(map[uint8]network.Engine),
	}

	return o, nil
}

// Ready returns a channel that will close when the network stack is ready.
func (n *Network) Ready() <-chan struct{} {
	ready := make(chan struct{})
	n.mw.Start(n)
	go func() {
		for n.state.Count() < 1 {
			time.Sleep(100 * time.Millisecond)
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
	nodeIDs := n.state.Peers().IDs()
	nodeIDs = append(nodeIDs, n.com.Me().ID)
	nodes, err := n.com.Select(filter.Not(filter.ID(nodeIDs...)))
	if err != nil {
		return "", errors.Wrap(err, "could not get nodes")
	}

	// if we don't have nodes available, we can't do anything
	if len(nodes) == 0 {
		return "", errors.New("no nodes available")
	}

	// select a random node from the list
	node := nodes[rand.Int()%len(nodes)]

	return node.Address, nil
}

// HandleHandshake implements a callback to validate new connections on the
// overlay layer, allowing us to implement peer authentication on first contact.
func (n *Network) Handshake(conn Connection) (string, error) {

	// initialize our own authentication message
	out := &trickle.Auth{
		NodeID: n.com.Me().ID,
	}
	err := conn.Send(out)
	if err != nil {
		return "", errors.Wrap(err, "could not send outgoing auth")
	}

	// read their authentication message
	msg, err := conn.Receive()
	if err != nil {
		return "", errors.Wrap(err, "could not receive incoming auth")
	}

	// type assert the response
	in, ok := msg.(*trickle.Auth)
	if !ok {
		return "", errors.Errorf("invalid type for incoming auth (%T)", msg)
	}

	// check if this node is ourselves
	if in.NodeID == n.com.Me().ID {
		return "", errors.New("connections to self forbidden")
	}

	// check if we are already connected to this peer
	ok = n.state.Alive(in.NodeID)
	if ok {
		return "", errors.Errorf("node already connected (%s)", in.NodeID)
	}

	// check if the committee has a node with the given ID
	_, err = n.com.Get(in.NodeID)
	if err != nil {
		return "", errors.Errorf("unknown nodeID (%s)", in.NodeID)
	}

	// initialize the peer state using the address as its ID
	n.state.Up(in.NodeID)

	return in.NodeID, nil
}

// Cleanup implements a callback to handle peers that have been dropped
// by the middleware layer.
func (n *Network) Cleanup(node string) error {

	// drop the peer state using the ID we registered
	n.state.Down(node)

	return nil
}

// Receive provides a callback to handle the incoming message from the
// given peer.
func (n *Network) Receive(peerID string, msg interface{}) error {
	var err error
	switch m := msg.(type) {
	case *trickle.Announce:
		err = n.processAnnounce(peerID, m)
	case *trickle.Request:
		err = n.processRequest(peerID, m)
	case *trickle.Response:
		err = n.processResponse(peerID, m)
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

	// if we don't have the engine, use an engine specific payload hash
	engine, ok := n.engines[engineID]
	if !ok {
		return nil, nil, errors.Errorf("missing engine on pack (%d)", engineID)
	}

	// try to identify using engine, otherwise use same engine specific hash
	eventID, err := engine.Identify(event)
	if err != nil {
		sip := siphash.New([]byte("trickleengine" + fmt.Sprintf("%03d", engineID)))
		return sip.Sum(payload), payload, nil
	}

	return eventID, payload, nil
}

// submit will submit the given event for the given engine to the overlay layer
// for processing; it is used by engines through conduits.
func (n *Network) submit(engineID uint8, event interface{}, targetIDs ...string) error {

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
func (n *Network) gossip(engineID uint8, eventID []byte, payload []byte, targetIDs ...string) error {

	// check if the event is already in the cache
	ok := n.cache.Has(engineID, eventID)
	if ok {
		return nil
	}

	// add gossip message to cache
	res := &trickle.Response{
		EngineID:  engineID,
		EventID:   eventID,
		OriginID:  n.com.Me().ID,
		TargetIDs: targetIDs,
		Payload:   payload,
	}
	n.cache.Set(engineID, eventID, res)

	// get all peers that haven't seen it yet
	peerIDs := n.state.Peers(Not(Seen(eventID))).IDs()
	if len(peerIDs) == 0 {
		return nil
	}

	// announce the event to the selected peers
	ann := &trickle.Announce{
		EngineID: engineID,
		EventID:  eventID,
	}
	err := n.send(ann, peerIDs...)
	if err != nil {
		return errors.Wrap(err, "could not send announce")
	}

	return nil
}

// processAnnounce will check if we have already seen the announced event and
// request it if it's new for us.
func (n *Network) processAnnounce(peerID string, ann *trickle.Announce) error {

	// remember that this peer has this event
	n.state.Seen(peerID, ann.EventID)

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
	err := n.mw.Send(peerID, req)
	if err != nil {
		return errors.Wrap(err, "could not send request")
	}

	return nil
}

// processRequest will process the given request and fulfill it.
func (n *Network) processRequest(peerID string, req *trickle.Request) error {

	// check to find ID and payload in cache
	res, ok := n.cache.Get(req.EngineID, req.EventID)
	if !ok {
		return nil
	}

	// send the response message to the peer
	err := n.mw.Send(peerID, res)
	if err != nil {
		return errors.Wrap(err, "could not send gossip")
	}

	return nil
}

// processResponse will process the payload of an event sent to us by a peer.
func (n *Network) processResponse(peerID string, res *trickle.Response) error {

	// mark this event as seen for this peer (usually redundant)
	n.state.Seen(peerID, res.EventID)

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
	ok = n.isfor(n.com.Me().ID, res)
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

func (n *Network) send(msg interface{}, peerIDs ...string) error {

	// send the message through the peer connection
	var result *multierror.Error
	for _, peerID := range peerIDs {
		err := n.mw.Send(peerID, msg)
		if err != nil {
			result = multierror.Append(result, err)
		}
	}

	return result.ErrorOrNil()
}

func (n *Network) isfor(nodeID string, res *trickle.Response) bool {

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
