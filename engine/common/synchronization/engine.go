// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package synchronization

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/storage"
)

// TODO: turn into configuration parameters
const (
	pollInterval  = 10 * time.Second
	scanInterval  = time.Second
	retryInterval = 10 * time.Second
	maxAttempts   = 3
	maxSize       = 64
	maxRequests   = 2
)

// Engine is the synchronization engine, responsible for synchronizing chain state.
type Engine struct {
	sync.Mutex
	unit     *engine.Unit
	log      zerolog.Logger
	me       module.Local
	state    protocol.State
	con      network.Conduit
	blocks   storage.Blocks
	comp     network.Engine // compliance engine
	heights  map[uint64]*Status
	blockIDs map[flow.Identifier]*Status
}

// New creates a new consensus propagation engine.
func New(
	log zerolog.Logger,
	net module.Network,
	me module.Local,
	state protocol.State,
	blocks storage.Blocks,
	comp network.Engine,
) (*Engine, error) {

	// initialize the propagation engine with its dependencies
	e := &Engine{
		unit:     engine.NewUnit(),
		log:      log.With().Str("engine", "consensus").Logger(),
		me:       me,
		state:    state,
		blocks:   blocks,
		comp:     comp,
		heights:  make(map[uint64]*Status),
		blockIDs: make(map[flow.Identifier]*Status),
	}

	// register the engine with the network layer and store the conduit
	con, err := net.Register(engine.ProtocolSynchronization, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine: %w", err)
	}
	e.con = con

	return e, nil
}

// Ready returns a ready channel that is closed once the engine has fully
// started. For consensus engine, this is true once the underlying consensus
// algorithm has started.
func (e *Engine) Ready() <-chan struct{} {
	e.unit.Launch(e.checkLoop)
	return e.unit.Ready()
}

// Done returns a done channel that is closed once the engine has fully stopped.
// For the consensus engine, we wait for hotstuff to finish.
func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done()
}

// SubmitLocal submits an event originating on the local node.
func (e *Engine) SubmitLocal(event interface{}) {
	e.Submit(e.me.NodeID(), event)
}

// Submit submits the given event from the node with the given origin ID
// for processing in a non-blocking manner. It returns instantly and logs
// a potential processing error internally when done.
func (e *Engine) Submit(originID flow.Identifier, event interface{}) {
	e.unit.Launch(func() {
		err := e.Process(originID, event)
		if err != nil {
			e.log.Error().Err(err).Msg("could not process submitted event")
		}
	})
}

// ProcessLocal processes an event originating on the local node.
func (e *Engine) ProcessLocal(event interface{}) error {
	return e.Process(e.me.NodeID(), event)
}

// Process processes the given event from the node with the given origin ID in
// a blocking manner. It returns the potential processing error when done.
func (e *Engine) Process(originID flow.Identifier, event interface{}) error {
	return e.unit.Do(func() error {
		return e.process(originID, event)
	})
}

// RequestBlock is an external provider allowing users to request block downloads
// by block ID.
func (e *Engine) RequestBlock(blockID flow.Identifier) {
	e.queueByBlockID(blockID)
}

// process processes events for the propagation engine on the consensus node.
func (e *Engine) process(originID flow.Identifier, event interface{}) error {
	switch ev := event.(type) {
	case *messages.SyncRequest:
		return e.onSyncRequest(originID, ev)
	case *messages.SyncResponse:
		return e.onSyncResponse(originID, ev)
	case *messages.RangeRequest:
		return e.onRangeRequest(originID, ev)
	case *messages.BatchRequest:
		return e.onBatchRequest(originID, ev)
	case *messages.BlockResponse:
		return e.onBlockResponse(originID, ev)
	default:
		return fmt.Errorf("invalid event type (%T)", event)
	}
}

// onSyncRequest processes an outgoing handshake; if we have a higher height, we
// inform the other node of it, so they can organize their block downloads. If
// we have a lower height, we add the difference to our own download queue.
func (e *Engine) onSyncRequest(originID flow.Identifier, req *messages.SyncRequest) error {

	// get the header at the latest finalized state
	final, err := e.state.Final().Head()
	if err != nil {
		return fmt.Errorf("could not get finalized head: %w", err)
	}

	// compare the height and see if we can bail early
	// TODO: we probably want to add some tolerance here
	if req.Height == final.Height {
		return nil
	}

	// if we are behind, we want to sync the missing blocks
	if req.Height > final.Height {
		for height := final.Height + 1; height <= req.Height; height++ {
			e.queueByHeight(height)
		}
		return nil
	}

	// create the response and send
	res := &messages.SyncResponse{
		Height: final.Height,
		Nonce:  req.Nonce,
	}
	err = e.con.Submit(res, originID)
	if err != nil {
		return fmt.Errorf("could not send sync response: %w", err)
	}

	return nil
}

// onSyncResponse processes a synchronization response.
func (e *Engine) onSyncResponse(originID flow.Identifier, res *messages.SyncResponse) error {

	// get the header at the latest finalized state
	head, err := e.state.Final().Head()
	if err != nil {
		return fmt.Errorf("could not get finalized head: %w", err)
	}

	// if the height is the same or smaller, there is nothing to do
	if res.Height <= head.Height {
		return nil
	}

	// mark the missing height
	for height := head.Height + 1; height <= res.Height; height++ {
		e.queueByHeight(height)
	}

	return nil
}

// onRangeRequest processes a request for a range of blocks by height.
func (e *Engine) onRangeRequest(originID flow.Identifier, req *messages.RangeRequest) error {

	// get the latest final state to know if we can fulfill the request
	head, err := e.state.Final().Head()
	if err != nil {
		return fmt.Errorf("could not get finalized head: %w", err)
	}

	// currently, we use `math.MaxUint64` as sentinel to indicate we want all finalized
	// blocks, starting from the from field
	if req.ToHeight == math.MaxUint64 {
		req.ToHeight = head.Height
	}

	// if we don't have the full range, or there is nothing to send, bail
	if head.Height < req.ToHeight || req.FromHeight > req.ToHeight {
		return nil
	}

	// get all of the blocks, one by one
	blocks := make([]*flow.Block, 0, req.ToHeight-req.FromHeight+1)
	for height := req.FromHeight; height <= req.ToHeight; height++ {
		block, err := e.blocks.ByHeight(height)
		if err != nil {
			return fmt.Errorf("could not get block for height (%d): %w", height, err)
		}
		blocks = append(blocks, block)
	}

	// send the response
	res := &messages.BlockResponse{
		Nonce:  req.Nonce,
		Blocks: blocks,
	}
	err = e.con.Submit(res, originID)
	if err != nil {
		return fmt.Errorf("could not send range response: %w", err)
	}

	return nil
}

// onBatchRequest processes a request for a specific block by block ID.
func (e *Engine) onBatchRequest(originID flow.Identifier, req *messages.BatchRequest) error {

	// TODO: de-duplicate the elements in `req.BlockIDs` to prevent an uplink  exhaustion attack (e.g. a request to send 500 times the same block).

	// try to get all the blocks by ID
	blocks := make([]*flow.Block, 0, len(req.BlockIDs))
	for _, blockID := range req.BlockIDs {
		block, err := e.blocks.ByID(blockID)
		if err != nil {
			return fmt.Errorf("could not get block by ID (%x): %w", blockID, err)
		}
		blocks = append(blocks, block)
	}

	// build the response
	res := &messages.BlockResponse{
		Nonce:  req.Nonce,
		Blocks: blocks,
	}
	err := e.con.Submit(res, originID)
	if err != nil {
		return fmt.Errorf("could not send batch response: %w", err)
	}

	return nil
}

// onBlockResponse processes a response containing a specifically requested block.
func (e *Engine) onBlockResponse(originID flow.Identifier, req *messages.BlockResponse) error {

	// process the blocks one by one
	for _, block := range req.Blocks {
		e.processIncomingBlock(block)
	}

	return nil
}

// queueByHeight queues a request for the finalized block at the given height.
func (e *Engine) queueByHeight(height uint64) {
	e.Lock()
	defer e.Unlock()

	// if we already have the height, skip
	_, ok := e.heights[height]
	if ok {
		return
	}

	// TODO: we might want to double check if we don't have the height finalized yet

	// create the status and add to map
	status := Status{
		Queued:    time.Now(),  // this can be used for timing out
		Requested: time.Time{}, // this can be used for both retries & timeouts
		Attempts:  0,           // this can be used to abort after certain amount of retries
	}
	e.heights[height] = &status
}

// queueByBlockID queues a request for a block by block ID.
func (e *Engine) queueByBlockID(blockID flow.Identifier) {
	e.Lock()
	defer e.Unlock()

	// if we already have the blockID, skip
	_, ok := e.blockIDs[blockID]
	if ok {
		return
	}

	// TODO: we might want to double check that we don't have the block yet

	// create the status and add to map
	status := Status{
		Queued:    time.Now(),
		Requested: time.Time{},
		Attempts:  0,
	}
	e.blockIDs[blockID] = &status
}

// processIncoming processes an incoming block, so we can take into account the
// overlap between block IDs and heights.
func (e *Engine) processIncomingBlock(block *flow.Block) {
	e.Lock()
	defer e.Unlock()

	// check if we still need to process this height or it is stale already
	blockID := block.ID()
	_, wantHeight := e.heights[block.Height]
	_, wantBlockID := e.blockIDs[blockID]
	if !wantHeight && !wantBlockID {
		return
	}

	// delete from the queue and forward
	delete(e.heights, block.Height)
	delete(e.blockIDs, blockID)
	e.comp.SubmitLocal(block)
}

// checkLoop will regularly scan for items that need requesting.
func (e *Engine) checkLoop() {
	poll := time.NewTicker(pollInterval)
	scan := time.NewTicker(scanInterval)

CheckLoop:
	for {
		select {
		case <-e.unit.Quit():
			break CheckLoop
		case <-poll.C:
			err := e.pollHeight()
			if err != nil {
				e.log.Error().Err(err).Msg("could not poll heights")
			}
		case <-scan.C:
			err := e.scanPending()
			if err != nil {
				e.log.Error().Err(err).Msg("could not execute requests")
			}
		}
	}

	// some minor cleanup
	scan.Stop()
	poll.Stop()
}

// pollHeight will send a synchronization request to three random nodes.
func (e *Engine) pollHeight() error {

	// get the last finalized header
	final, err := e.state.Final().Head()
	if err != nil {
		return fmt.Errorf("could not get last finalized header: %w", err)
	}

	// get all of the consensus nodes from the state
	identities, err := e.state.Final().Identities(
		filter.HasRole(flow.RoleConsensus),
		filter.Not(filter.HasNodeID(e.me.NodeID())),
	)
	if err != nil {
		return fmt.Errorf("could not send get consensus identities: %w", err)
	}

	// send the request for synchronization
	for _, targetID := range identities.Sample(3).NodeIDs() {
		req := &messages.SyncRequest{
			Nonce:  rand.Uint64(),
			Height: final.Height,
		}
		err := e.con.Submit(req, targetID)
		if err != nil {
			return fmt.Errorf("could not send sync request: %w", err)
		}
	}

	return nil
}

// scanPending will check which items shall be requested.
func (e *Engine) scanPending() error {
	e.Lock()
	defer e.Unlock()

	// TODO: we will probably want to limit the maximum amount of in-flight
	// requests and maximum amount of blocks requested at the same time here;
	// for now, we just ignore that problem, but once we do, we should always
	// prioritize range requests over batch requests

	// we simply use the same timestamp for each scan, that should be fine
	now := time.Now()

	// create a list of all height requests that should be sent
	var heights []uint64
	for height, status := range e.heights {

		// if the last request is young enough, skip
		cutoff := status.Requested.Add(retryInterval)
		if now.Before(cutoff) {
			continue
		}

		// if we reached maximum number of attempts, delete
		if status.Attempts >= maxAttempts {
			delete(e.heights, height)
			continue
		}

		// otherwise, append to heights to be requested
		status.Requested = time.Now()
		status.Attempts++
		heights = append(heights, height)
	}

	// create list of all the block IDs blocks that are missing
	var blockIDs []flow.Identifier
	for blockID, status := range e.blockIDs {

		// if the last request is young enough, skip
		cutoff := status.Requested.Add(retryInterval)
		if now.Before(cutoff) {
			continue
		}

		// if we reached the maximum number of attempts for a queue item, drop
		if status.Attempts >= maxAttempts {
			delete(e.blockIDs, blockID)
			continue
		}

		// otherwise, append to blockIDs to be requested
		status.Requested = now
		status.Attempts++
		blockIDs = append(blockIDs, blockID)
	}

	// bail out if nothing to send
	if len(heights) == 0 && len(blockIDs) == 0 {
		return nil
	}

	// send the requests
	err := e.sendRequests(heights, blockIDs)
	if err != nil {
		return fmt.Errorf("could not send requests: %w", err)
	}

	return nil
}

// sendRequests will divide the given heights and block IDs appropriately and
// request the desired blocks.
func (e *Engine) sendRequests(heights []uint64, blockIDs []flow.Identifier) error {

	// get all of the consensus nodes from the state
	// NOTE: does this still make sense for the follower?
	identities, err := e.state.Final().Identities(
		filter.HasRole(flow.RoleConsensus),
		filter.Not(filter.HasNodeID(e.me.NodeID())),
	)
	if err != nil {
		return fmt.Errorf("could not send get consensus identities: %w", err)
	}

	// sort the heights so we can build contiguous ranges more easily
	sort.Slice(heights, func(i int, j int) bool {
		return heights[i] < heights[j]
	})

	// build contiguous height ranges with maximum batch size
	start := uint64(0)
	end := uint64(0)
	var ranges []Range
	for _, height := range heights {

		// check if we start a range
		if start == 0 {
			start = height
			end = height
			continue
		}

		// first check triggers when we have non-contiguous heights
		// second check triggers when we reached the maximum range size
		// in both cases, we break it up into an additional range
		if height > end+1 || height >= start+maxSize {
			r := Range{From: start, To: end}
			ranges = append(ranges, r)
			start = height
			end = height
			continue
		}

		// otherwise, we simply increase the end by one
		end++
	}

	// for each range, send a request until reaching max number
	totalRequests := 0
	for _, r := range ranges {

		// send the request first
		req := &messages.RangeRequest{
			Nonce:      rand.Uint64(),
			FromHeight: r.From,
			ToHeight:   r.To,
		}
		err := e.con.Submit(req, identities.Sample(3).NodeIDs()...)
		if err != nil {
			return fmt.Errorf("could not send range request: %w", err)
		}

		// check if we reached the maximum number of requests for this period
		totalRequests++
		if totalRequests >= maxRequests {
			return nil
		}
	}

	// split the block IDs into maximum sized requests
	for from := 0; from < len(blockIDs); from += maxSize {

		// make sure last range is not out of bounds
		to := from + maxSize
		if to > len(blockIDs) {
			to = len(blockIDs)
		}

		// create the block IDs slice
		requestIDs := blockIDs[from:to]
		req := &messages.BatchRequest{
			Nonce:    rand.Uint64(),
			BlockIDs: requestIDs,
		}
		err := e.con.Submit(req, identities.Sample(3).NodeIDs()...)
		if err != nil {
			return fmt.Errorf("could not send batch request: %w", err)
		}

		// check if we reached maximum requests
		totalRequests++
		if totalRequests >= maxRequests {
			return nil
		}
	}

	return nil
}
