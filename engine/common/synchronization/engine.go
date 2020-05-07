// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package synchronization

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/events"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/storage"
)

// Engine is the synchronization engine, responsible for synchronizing chain state.
type Engine struct {
	unit     *engine.Unit
	log      zerolog.Logger
	me       module.Local
	state    protocol.State
	con      network.Conduit
	blocks   storage.Blocks
	comp     network.Engine // compliance engine
	heights  map[uint64]*Status
	blockIDs map[flow.Identifier]*Status

	// config parameters
	pollInterval  time.Duration
	scanInterval  time.Duration
	retryInterval time.Duration
	tolerance     uint // tolerance determines how big of a difference in block heights we tolerated before actively syncing with range requests
	maxAttempts   uint // maxAttempts is the maximum number of attempts to sync we make for each requested block/height before discarding
	maxSize       uint // maxSize is the maximum number of blocks we request in the same block request message
	maxRequests   uint // maxRequests is the maximum number of requests we send during each scanning period
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

		pollInterval:  8 * time.Second,
		scanInterval:  1 * time.Second,
		retryInterval: 4 * time.Second,
		tolerance:     10,
		maxAttempts:   5,
		maxSize:       64,
		maxRequests:   3,
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
	e.unit.Lock()
	defer e.unit.Unlock()

	e.queueByBlockID(blockID)
}

// process processes events for the propagation engine on the consensus node.
func (e *Engine) process(originID flow.Identifier, event interface{}) error {

	// process one event at a time for now
	e.unit.Lock()
	defer e.unit.Unlock()

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

	// if we are within the tolerance threshold we do nothing
	lower := final.Height - uint64(e.tolerance)
	if lower > final.Height { // overflow check
		lower = 0
	}
	upper := final.Height + uint64(e.tolerance)
	if req.Height >= lower && req.Height <= upper {
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
	final, err := e.state.Final().Head()
	if err != nil {
		return fmt.Errorf("could not get finalized head: %w", err)
	}

	// if we are within the tolerance threshold we do nothing
	upper := final.Height + uint64(e.tolerance)
	if res.Height <= upper {
		return nil
	}

	// mark the missing height
	for height := final.Height + 1; height <= res.Height; height++ {
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

	// if we don't have anything to send, we can bail right away
	if head.Height < req.FromHeight || req.FromHeight > req.ToHeight {
		return nil
	}

	// get all of the blocks, one by one
	blocks := make([]*flow.Block, 0, req.ToHeight-req.FromHeight+1)
	for height := req.FromHeight; height <= req.ToHeight; height++ {
		block, err := e.blocks.ByHeight(height)
		if errors.Is(err, storage.ErrNotFound) {
			e.log.Debug().Uint64("height", height).Msg("skipping unknown heights")
			break
		}
		if err != nil {
			return fmt.Errorf("could not get block for height (%d): %w", height, err)
		}
		blocks = append(blocks, block)
	}

	// if there are no blocks to send, skip network message
	if len(blocks) == 0 {
		e.log.Debug().Msg("skipping empty range response")
		return nil
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

	// we should bail and send nothing on empty request
	if len(req.BlockIDs) == 0 {
		return nil
	}

	// deduplicate the block IDs in the batch request
	blockIDs := make(map[flow.Identifier]struct{})
	for _, blockID := range req.BlockIDs {
		blockIDs[blockID] = struct{}{}
	}

	// try to get all the blocks by ID
	blocks := make([]*flow.Block, 0, len(blockIDs))
	for blockID := range blockIDs {
		block, err := e.blocks.ByID(blockID)
		if errors.Is(err, storage.ErrNotFound) {
			e.log.Debug().Hex("block_id", blockID[:]).Msg("skipping unknown block")
			continue
		}
		if err != nil {
			return fmt.Errorf("could not get block by ID (%s): %w", blockID, err)
		}
		blocks = append(blocks, block)
	}

	// if there are no blocks to send, skip network message
	if len(blocks) == 0 {
		e.log.Debug().Msg("skipping empty batch response")
		return nil
	}

	// send the response
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
func (e *Engine) onBlockResponse(originID flow.Identifier, res *messages.BlockResponse) error {

	// process the blocks one by one
	for _, block := range res.Blocks {
		e.processIncomingBlock(originID, block)
	}

	return nil
}

// queueByHeight queues a request for the finalized block at the given height.
func (e *Engine) queueByHeight(height uint64) {

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
func (e *Engine) processIncomingBlock(originID flow.Identifier, block *flow.Block) {

	// check if we still need to process this block or it is stale already
	blockID := block.ID()
	_, wantHeight := e.heights[block.Header.Height]
	_, wantBlockID := e.blockIDs[blockID]
	if !wantHeight && !wantBlockID {
		return
	}

	// delete from the queue and forward
	delete(e.heights, block.Header.Height)
	delete(e.blockIDs, blockID)

	synced := &events.SyncedBlock{
		OriginID: originID,
		Block:    block,
	}

	e.comp.SubmitLocal(synced)
}

// checkLoop will regularly scan for items that need requesting.
func (e *Engine) checkLoop() {
	poll := time.NewTicker(e.pollInterval)
	scan := time.NewTicker(e.scanInterval)

CheckLoop:
	for {
		select {
		case <-e.unit.Quit():
			break CheckLoop
		case <-poll.C:
			err := e.pollHeight()
			if err != nil {
				e.log.Error().Err(err).Msg("could not poll heights")
				continue
			}
		case <-scan.C:
			heights, blockIDs, err := e.scanPending()
			if err != nil {
				e.log.Error().Err(err).Msg("could not scan pending")
				continue
			}
			err = e.sendRequests(heights, blockIDs)
			if err != nil {
				e.log.Error().Err(err).Msg("could not send requests")
				continue
			}
		}
	}

	// some minor cleanup
	scan.Stop()
	poll.Stop()
}

// pollHeight will send a synchronization request to three random nodes.
func (e *Engine) pollHeight() error {
	e.unit.Lock()
	defer e.unit.Unlock()

	// get the last finalized header
	final, err := e.state.Final().Head()
	if err != nil {
		return fmt.Errorf("could not get last finalized header: %w", err)
	}

	// get all of the consensus nodes from the state
	identities, err := e.state.Final().Identities(filter.And(
		filter.HasRole(flow.RoleConsensus),
		filter.Not(filter.HasNodeID(e.me.NodeID())),
	))
	if err != nil {
		return fmt.Errorf("could not send get consensus identities: %w", err)
	}

	var errs error
	// send the request for synchronization
	for _, targetID := range identities.Sample(3).NodeIDs() {
		req := &messages.SyncRequest{
			Nonce:  rand.Uint64(),
			Height: final.Height,
		}
		err := e.con.Submit(req, targetID)
		if err != nil {
			errs = multierror.Append(errs, fmt.Errorf("could not send sync request: %w", err))
		}
	}

	return errs
}

// scanPending will check which items shall be requested.
func (e *Engine) scanPending() ([]uint64, []flow.Identifier, error) {
	e.unit.Lock()
	defer e.unit.Unlock()

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
		cutoff := status.Requested.Add(e.retryInterval)
		if now.Before(cutoff) {
			continue
		}

		// if we reached maximum number of attempts, delete
		if status.Attempts >= e.maxAttempts {
			delete(e.heights, height)
			continue
		}

		// otherwise, append to heights to be requested
		heights = append(heights, height)
	}

	// create list of all the block IDs blocks that are missing
	var blockIDs []flow.Identifier
	for blockID, status := range e.blockIDs {

		// if the last request is young enough, skip
		cutoff := status.Requested.Add(e.retryInterval)
		if now.Before(cutoff) {
			continue
		}

		// if we reached the maximum number of attempts for a queue item, drop
		if status.Attempts >= e.maxAttempts {
			delete(e.blockIDs, blockID)
			continue
		}

		// otherwise, append to blockIDs to be requested
		blockIDs = append(blockIDs, blockID)
	}

	return heights, blockIDs, nil
}

// sendRequests will divide the given heights and block IDs appropriately and
// request the desired blocks.
func (e *Engine) sendRequests(heights []uint64, blockIDs []flow.Identifier) error {
	e.unit.Lock()
	defer e.unit.Unlock()

	// get all of the consensus nodes from the state
	// NOTE: we want to request from consensus nodes even on other node roles
	identities, err := e.state.Final().Identities(filter.And(
		filter.HasRole(flow.RoleConsensus),
		filter.Not(filter.HasNodeID(e.me.NodeID())),
	))
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
	for index, height := range heights {

		// on the first iteration, we set the start pointer, so we don't need to
		// guard the for loop when heights is empty
		if index == 0 {
			start = height
		}

		// we always forward the end pointer to the new height
		end = height

		// if we have the end of the loop, we always create one final range
		if index >= len(heights)-1 {
			r := Range{From: start, To: end}
			ranges = append(ranges, r)
			break
		}

		// at this point, we will have a next height as iteration will continue
		nextHeight := heights[index+1]

		// if we have reached the maximum size for a range, we create the range
		// and forward the start pointer to the next height
		rangeSize := end - start + 1
		if rangeSize >= uint64(e.maxSize) {
			r := Range{From: start, To: end}
			ranges = append(ranges, r)
			start = nextHeight
			continue
		}

		// if end is more than one smaller than the next height, we have a gap
		// next, so we create a range and forward the start pointer
		if nextHeight > end+1 {
			r := Range{From: start, To: end}
			ranges = append(ranges, r)
			start = nextHeight
			continue
		}
	}

	// for each range, send a request until reaching max number
	totalRequests := uint(0)
	for _, ran := range ranges {

		// send the request first
		req := &messages.RangeRequest{
			Nonce:      rand.Uint64(),
			FromHeight: ran.From,
			ToHeight:   ran.To,
		}
		err := e.con.Submit(req, identities.Sample(3).NodeIDs()...)
		if err != nil {
			return fmt.Errorf("could not send range request: %w", err)
		}

		// mark all of the heights as requested; these should always be there as
		// we call this right after the scan and nothing else can delete keys
		for height := ran.From; height <= ran.To; height++ {
			e.heights[height].Requested = time.Now()
			e.heights[height].Attempts++
		}

		// check if we reached the maximum number of requests for this period
		totalRequests++
		if totalRequests >= e.maxRequests {
			return nil
		}
	}

	// split the block IDs into maximum sized requests
	for from := 0; from < len(blockIDs); from += int(e.maxSize) {

		// make sure last range is not out of bounds
		to := from + int(e.maxSize)
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

		// mark all of the blocks as requested; these should always be there as
		// we call this right after the scan and nothing else can delete keys
		for _, blockID := range requestIDs {
			e.blockIDs[blockID].Requested = time.Now()
			e.blockIDs[blockID].Attempts++
		}

		// check if we reached maximum requests
		totalRequests++
		if totalRequests >= e.maxRequests {
			return nil
		}
	}

	// TODO: when not requesting batches/ranges, make sure we don't add an
	// attempt and we don't restart the retry timeout

	return nil
}
