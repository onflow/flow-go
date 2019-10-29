// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package propagation

import (
	"bytes"
	"math/rand"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/model/collection"
	"github.com/dapperlabs/flow-go/model/consensus"
	"github.com/dapperlabs/flow-go/model/filter"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/network"
)

const (
	// CirculatorEngine is The engine code for CirculatorEngine
	CirculatorEngine = 10
)

// Engine implements the circulation engine of the consensus node, responsible
// for making sure collections are propagated to all consensus nodes within the
// flow system.
type Engine struct {
	log     zerolog.Logger   // used to log relevant actions with context
	com     module.Committee // holds the node identity table for the network
	con     network.Conduit  // used to talk to other nodes on the network
	pool    Mempool          // holds guaranteed collections in memory
	vol     Volatile         // holds volatile information on collection exchange
	polling time.Duration    // interval at which we poll for mempool contents
	wg      *sync.WaitGroup  // used to wait on cleanup upon shutdown
	once    *sync.Once       // used to only close the done channel once
	stop    chan struct{}    // used as a signal to indicate engine shutdown
}

// NewEngine creates a new collection propagation engine.
func NewEngine(log zerolog.Logger, net module.Network, com module.Committee, pool Mempool, vol Volatile) (*Engine, error) {

	// initialize the propagation engine with its dependencies
	e := &Engine{
		log:     log,
		com:     com,
		pool:    pool,
		vol:     vol,
		polling: 20 * time.Second,
		wg:      &sync.WaitGroup{},
		once:    &sync.Once{},
		stop:    make(chan struct{}),
	}

	// register the engine with the network layer and store the conduit
	con, err := net.Register(CirculatorEngine, e)
	if err != nil {
		return nil, errors.Wrap(err, "could not register engine")
	}

	e.con = con

	return e, nil
}

// Ready returns a ready channel that is closed once the engine has fully
// started. For the propagation engine, we consider the engine up and running
// when we have received the first guaranteed collection for the memory pool.
// We thus start polling and wait until the memory pool has a size of at least
// one.
func (e *Engine) Ready() <-chan struct{} {
	ready := make(chan struct{})
	e.wg.Add(1)
	go e.pollSnapshots()
	go func() {
		for e.pool.Size() < 1 {
			time.Sleep(100 * time.Millisecond)
		}
		close(ready)
	}()
	return ready
}

// Done returns a done channel that is closed once the engine has fully stopped.
// It closes the internal stop channel to signal all running go routines and
// then waits for them to finish using the wait group.
func (e *Engine) Done() <-chan struct{} {
	done := make(chan struct{})
	e.once.Do(func() {
		close(e.stop)
	})
	go func() {
		e.wg.Wait()
		close(done)
	}()
	return done
}

// Identify uniquely identifies the given event within the context of the
// propagation engine. In this case, it only recognized guaranteed collections
// and returns the canonical hash of the related collection.
func (e *Engine) Identify(event interface{}) ([]byte, error) {
	switch ev := event.(type) {
	case *collection.GuaranteedCollection:
		return ev.Hash, nil
	default:
		return nil, errors.Errorf("hash not implement for type (%T)", event)
	}
}

// Retrieve retrieves the event with the given unique ID, as provided by the
// identify function. In this case, it only retrieves guaranteed collections
// using the canonical hash of the related collection.
func (e *Engine) Retrieve(eventID []byte) (interface{}, error) {
	coll, err := e.pool.Get(eventID)
	if err != nil {
		return nil, errors.Wrap(err, "could not get fingerprint from mempool")
	}
	return coll, nil
}

// Process processes the given propagation engine event. Events that are given
// to this function originate within the propagation engine on the node with the
// given origin ID.
func (e *Engine) Process(originID string, event interface{}) error {
	var err error
	switch ev := event.(type) {
	case *consensus.SnapshotRequest:
		err = e.onSnapshotRequest(originID, ev)
	case *consensus.SnapshotResponse:
		err = e.onSnapshotResponse(originID, ev)
	case *consensus.MempoolRequest:
		err = e.onMempoolRequest(originID, ev)
	case *consensus.MempoolResponse:
		err = e.onMempoolResponse(originID, ev)
	case *collection.GuaranteedCollection:
		err = e.onGuaranteedCollection(originID, ev)
	default:
		err = errors.Errorf("invalid event type (%T)", event)
	}
	if err != nil {
		return errors.Wrap(err, "could not process event")
	}
	return nil
}

// SubmitGuaranteedCollection is used to submit new guaranteed collections from
// within the local node business logic. It can be used by other engines to
// introduce new guaranteed collections to the propagation process.
func (e *Engine) SubmitGuaranteedCollection(coll *collection.GuaranteedCollection) error {

	// first process the guaranteed collection to make sure it is actually valid
	err := e.processGuaranteedCollection(coll)
	if err != nil {
		return errors.Wrap(err, "could not process collection")
	}

	// then propagate the guaranteed collections to relevant nodes on the network
	err = e.propagateGuaranteedCollection(coll)
	if err != nil {
		return errors.Wrap(err, "could not broadcast collection")
	}

	return nil
}

// pollSnapshots will poll for snapshots from other peers at regular intervals.
func (e *Engine) pollSnapshots() {
	defer e.wg.Done()
Loop:
	for {
		select {

		case <-e.stop:
			break Loop

		case <-time.After(e.polling):
			err := e.propagateSnapshotRequest()
			if err != nil {
				e.log.Error().Err(err).Msg("could not request snapshot")
				continue
			}
		}
	}
}

// onSnapshotRequest is called when another node within the guaranteed
// collection propagation process requests a snapshot of our collection memory
// pool.
func (e *Engine) onSnapshotRequest(originID string, req *consensus.SnapshotRequest) error {

	e.log.Info().
		Str("origin_id", originID).
		Uint64("nonce", req.Nonce).
		Msg("snapshot request received")

	// check if our memory pool has the same hash, in which case there is no need
	// to send them the memory pool snapshot or exchange its contents
	hash := e.pool.Hash()
	if bytes.Equal(hash, req.MempoolHash) {
		return nil
	}

	// reply with our memory pool hash and the related nonce, which allows the
	// recipient to associate the reply and request contents of the memory pool
	// if it so desires
	res := &consensus.SnapshotResponse{
		Nonce:       req.Nonce,
		MempoolHash: hash,
	}
	err := e.con.Submit(res, originID)
	if err != nil {
		return errors.Wrap(err, "could not send snapshot response")
	}

	// TODO: we can keep track of the request and related responses by introducing
	// the nonce into our volatile state

	e.log.Info().
		Str("target_id", originID).
		Uint64("nonce", req.Nonce).
		Hex("mempool_hash", hash).
		Msg("snapshot response sent")

	return nil
}

// onSnapshotResponse is called when another node responds to our request for
// a snapshot of their collection memory pool.
func (e *Engine) onSnapshotResponse(originID string, res *consensus.SnapshotResponse) error {

	e.log.Info().
		Str("origin_id", originID).
		Uint64("nonce", res.Nonce).
		Hex("mempool_hash", res.MempoolHash).
		Msg("snapshot response received")

	// TODO: we can check whether we actually requested the snapshot by checking
	// the nonce against our volatile state

	// TODO: we can check whether we have already processed a response from this
	// node by checking the origin ID against our volatile state

	// TODO: we can keep track of which nodes have already sent us a response by
	// introducing the origin ID into our volatile state

	// TODO: we can check whether we already requested a memory pool with the
	// given hash by checking the memory pool hash versus our volatile state

	// send a memory pool request to the given node with the same nonce
	req := &consensus.MempoolRequest{
		Nonce: res.Nonce,
	}
	err := e.con.Submit(req, originID)
	if err != nil {
		return errors.Wrap(err, "could not send mempool request")
	}

	// TODO: we can keep track of which memory pool hashes we have already
	// requested by adding the memory pool hash into our volatile state

	e.log.Info().
		Str("target_id", originID).
		Uint64("nonce", res.Nonce).
		Msg("mempool request sent")

	return nil
}

// onMempoolRequest is called when another node requests the contents of our
// collection memory pool.
func (e *Engine) onMempoolRequest(originID string, req *consensus.MempoolRequest) error {

	e.log.Info().
		Str("origin_id", originID).
		Uint64("nonce", req.Nonce).
		Msg("mempool request received")

	// TODO: we can check whether we actually provided this snapshot by checking
	// the nonce against our volatile state

	// TODO: we can check if we have already sent the memory pool to this node
	// by checking the origin ID against our volatile state

	// send our memory pool contents to the node using the same nonce
	colls := e.pool.All()
	res := &consensus.MempoolResponse{
		Nonce:       req.Nonce,
		Collections: colls,
	}
	err := e.con.Submit(res, originID)
	if err != nil {
		return errors.Wrap(err, "could not send mempool response")
	}

	e.log.Info().
		Str("target_id", originID).
		Uint64("nonce", req.Nonce).
		Int("sent_collections", len(colls)).
		Msg("mempool response sent")

	return nil
}

// onMempoolResponse is called when another node sends us the contents of its
// collection memory pool.
func (e *Engine) onMempoolResponse(originID string, res *consensus.MempoolResponse) error {

	e.log.Info().
		Str("origin_id", originID).
		Uint64("nonce", res.Nonce).
		Int("received_collections", len(res.Collections)).
		Msg("mempool response received")

	// TODO: we can check whether we have an ongoing memory pool exchange with the
	// given  nonce by checking it against our volatile state

	// TODO: we can check whether we already processed a memory pool response from
	// this node by checking it against our volatile state

	added := 0
	var result *multierror.Error
	for _, coll := range res.Collections {
		err := e.processGuaranteedCollection(coll)
		if err != nil {
			result = multierror.Append(result, err)
			continue
		}
		added++
	}

	// TODO: we can keep track of which nodes we already received responses from
	// by adding it to our volatile state

	e.log.Info().
		Str("target_id", originID).
		Uint64("nonce", res.Nonce).
		Int("added_collections", added).
		Uint("mempool_size", e.pool.Size()).
		Msg("mempool response processed")

	return result.ErrorOrNil()
}

// onGuaranteedCollection is called when a new guaranteed collection is received
// from the given node on the network.
func (e *Engine) onGuaranteedCollection(originID string, coll *collection.GuaranteedCollection) error {

	e.log.Info().
		Str("origin_id", originID).
		Hex("collection_hash", coll.Hash).
		Msg("fingerprint message received")

	err := e.processGuaranteedCollection(coll)
	if err != nil {
		return errors.Wrap(err, "could not process collection")
	}

	e.log.Info().
		Str("origin_id", originID).
		Hex("collection_hash", coll.Hash).
		Uint("mempool_size", e.pool.Size()).
		Msg("guaranteed collection processed")

	return nil
}

// processGuaranteedCollection will process a guaranteed collection by
// validating it and adding it to our memory pool.
func (e *Engine) processGuaranteedCollection(coll *collection.GuaranteedCollection) error {

	// check if we already know the guaranteed collection with the given hash, in
	// which case we don't need to further process it
	ok := e.pool.Has(coll.Hash)
	if ok {
		return nil
	}

	// TODO: validate the guaranteed collection signature

	// add the guaranteed collection to our memory pool
	err := e.pool.Add(coll)
	if err != nil {
		return errors.Wrap(err, "could not add collection")
	}

	return nil
}

// propagateGuaranteedCollection will propagate the collection to the relevant
// nodes on the network.
func (e *Engine) propagateGuaranteedCollection(coll *collection.GuaranteedCollection) error {

	// select all the collection nodes on the network as our targets
	identities, err := e.com.Select(
		filter.Role("consensus"),
		filter.Not(filter.NodeID(e.com.Me().NodeID)),
	)
	if err != nil {
		return errors.Wrap(err, "could not select identities")
	}

	// send the guaranteed collection to all consensus identities
	targetIDs := identities.NodeIDs()
	err = e.con.Submit(coll, targetIDs...)
	if err != nil {
		return errors.Wrap(err, "could not push guaranteed collection")
	}

	e.log.Info().
		Strs("target_ids", targetIDs).
		Hex("collection_hash", coll.Hash).
		Msg("guaranteed collection propagated")

	return nil
}

// propagateSnapshotRequest will propagate a memory pool snapshot request to the
// relevant nodes on the network.
func (e *Engine) propagateSnapshotRequest() error {

	// select all the consensus nodes on the network to request snapshot
	identities, err := e.com.Select(
		filter.Role("consensus"),
		filter.Not(filter.NodeID(e.com.Me().NodeID)),
	)
	if err != nil {
		return errors.Wrap(err, "could not find consensus nodes")
	}

	// send the snapshot request to the selected nodes
	hash := e.pool.Hash()
	targetIDs := identities.NodeIDs()
	req := &consensus.SnapshotRequest{
		Nonce:       rand.Uint64(),
		MempoolHash: hash,
	}
	err = e.con.Submit(req, targetIDs...)
	if err != nil {
		return errors.Wrap(err, "could not send snapshot request")
	}

	e.log.Info().
		Uint64("nonce", req.Nonce).
		Strs("target_ids", targetIDs).
		Hex("mempool_hash", hash).
		Msg("snapshot request propagated")

	return nil
}
