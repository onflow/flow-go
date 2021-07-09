// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package synchronization

import (
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	clustermodel "github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/events"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	synccore "github.com/onflow/flow-go/module/synchronization"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/state/cluster"
	"github.com/onflow/flow-go/storage"
)

// Engine is the synchronization engine, responsible for synchronizing chain state.
type Engine struct {
	unit         *engine.Unit
	log          zerolog.Logger
	metrics      module.EngineMetrics
	me           module.Local
	participants flow.IdentityList
	state        cluster.State
	conduit      network.Conduit
	chainID      flow.ChainID
	blocks       storage.ClusterBlocks
	comp         network.Engine // compliance layer engine

	pollInterval time.Duration
	scanInterval time.Duration
	core         module.SyncCore
}

// New creates a new cluster synchronization engine.
func New(
	log zerolog.Logger,
	metrics module.EngineMetrics,
	net module.Network,
	me module.Local,
	participants flow.IdentityList,
	state cluster.State,
	blocks storage.ClusterBlocks,
	comp network.Engine,
	core module.SyncCore,
) (*Engine, error) {

	// initialize the propagation engine with its dependencies
	e := &Engine{
		unit:         engine.NewUnit(),
		log:          log.With().Str("engine", "cluster_synchronization").Logger(),
		metrics:      metrics,
		me:           me,
		participants: participants.Filter(filter.Not(filter.HasNodeID(me.NodeID()))),
		state:        state,
		blocks:       blocks,
		comp:         comp,
		core:         core,
		pollInterval: 8 * time.Second,
		scanInterval: 2 * time.Second,
	}

	chainID, err := state.Params().ChainID()
	if err != nil {
		return nil, fmt.Errorf("could not get chain ID: %w", err)
	}

	e.chainID = chainID

	// register the engine with the network layer and store the conduit
	conduit, err := net.Register(engine.ChannelSyncCluster(chainID), e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine: %w", err)
	}
	e.conduit = conduit

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
	return e.unit.Done(func() {
		err := e.conduit.Close()
		if err != nil {
			e.log.Error().Err(err).Msg("could not close conduit")
		}
	})
}

// SubmitLocal submits an event originating on the local node.
func (e *Engine) SubmitLocal(event interface{}) {
	e.unit.Launch(func() {
		err := e.process(e.me.NodeID(), event)
		if err != nil {
			engine.LogError(e.log, err)
		}
	})
}

// Submit submits the given event from the node with the given origin ID
// for processing in a non-blocking manner. It returns instantly and logs
// a potential processing error internally when done.
func (e *Engine) Submit(channel network.Channel, originID flow.Identifier, event interface{}) {
	e.unit.Launch(func() {
		err := e.Process(channel, originID, event)
		if err != nil {
			engine.LogError(e.log, err)
		}
	})
}

// ProcessLocal processes an event originating on the local node.
func (e *Engine) ProcessLocal(event interface{}) error {
	return e.unit.Do(func() error {
		return e.process(e.me.NodeID(), event)
	})
}

// Process processes the given event from the node with the given origin ID in
// a blocking manner. It returns the potential processing error when done.
func (e *Engine) Process(channel network.Channel, originID flow.Identifier, event interface{}) error {
	return e.unit.Do(func() error {
		return e.process(originID, event)
	})
}

// process processes events for the propagation engine on the consensus node.
func (e *Engine) process(originID flow.Identifier, event interface{}) error {

	switch ev := event.(type) {
	case *messages.SyncRequest:
		e.before(metrics.MessageSyncRequest)
		defer e.after(metrics.MessageSyncRequest)
		return e.onSyncRequest(originID, ev)
	case *messages.SyncResponse:
		e.before(metrics.MessageSyncResponse)
		defer e.after(metrics.MessageSyncResponse)
		return e.onSyncResponse(originID, ev)
	case *messages.RangeRequest:
		e.before(metrics.MessageRangeRequest)
		defer e.after(metrics.MessageRangeRequest)
		return e.onRangeRequest(originID, ev)
	case *messages.BatchRequest:
		e.before(metrics.MessageBatchRequest)
		defer e.after(metrics.MessageBatchRequest)
		return e.onBatchRequest(originID, ev)
	case *messages.ClusterBlockResponse:
		e.before(metrics.MessageClusterBlockResponse)
		defer e.after(metrics.MessageClusterBlockResponse)
		return e.onBlockResponse(originID, ev)
	default:
		return fmt.Errorf("invalid event type (%T)", event)
	}
}

func (e *Engine) before(msg string) {
	e.metrics.MessageReceived(metrics.EngineClusterSynchronization, msg)
	e.unit.Lock()
}

func (e *Engine) after(msg string) {
	e.unit.Unlock()
	e.metrics.MessageHandled(metrics.EngineClusterSynchronization, msg)
}

// onSyncRequest processes an outgoing handshake; if we have a higher height, we
// inform the other node of it, so they can organize their block downloads. If
// we have a lower height, we add the difference to our own download queue.
func (e *Engine) onSyncRequest(originID flow.Identifier, req *messages.SyncRequest) error {

	final, err := e.state.Final().Head()
	if err != nil {
		return fmt.Errorf("could not get finalized height: %w", err)
	}

	// queue any missing heights as needed
	e.core.HandleHeight(final, req.Height)

	// don't bother sending a response if we're within tolerance or if we're
	// behind the requester
	if e.core.WithinTolerance(final, req.Height) || req.Height > final.Height {
		return nil
	}

	// if we're sufficiently ahead of the requester, send a response
	res := &messages.SyncResponse{
		Height: final.Height,
		Nonce:  req.Nonce,
	}
	err = e.conduit.Unicast(res, originID)
	if err != nil {
		return fmt.Errorf("could not send sync response: %w", err)
	}

	e.metrics.MessageSent(metrics.EngineClusterSynchronization, metrics.MessageSyncResponse)

	return nil
}

// onSyncResponse processes a synchronization response.
func (e *Engine) onSyncResponse(originID flow.Identifier, res *messages.SyncResponse) error {

	final, err := e.state.Final().Head()
	if err != nil {
		return fmt.Errorf("could not get finalized height: %w", err)
	}

	e.core.HandleHeight(final, res.Height)
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
	blocks := make([]*clustermodel.Block, 0, req.ToHeight-req.FromHeight+1)
	for height := req.FromHeight; height <= req.ToHeight; height++ {
		block, err := e.blocks.ByHeight(height)
		if errors.Is(err, storage.ErrNotFound) {
			e.log.Error().Uint64("height", height).Msg("skipping unknown heights")
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
	res := &messages.ClusterBlockResponse{
		Nonce:  req.Nonce,
		Blocks: blocks,
	}
	err = e.conduit.Unicast(res, originID)
	if err != nil {
		return fmt.Errorf("could not send range response: %w", err)
	}

	e.metrics.MessageSent(metrics.EngineClusterSynchronization, metrics.MessageBlockResponse)

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
	blocks := make([]*clustermodel.Block, 0, len(blockIDs))
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
	res := &messages.ClusterBlockResponse{
		Nonce:  req.Nonce,
		Blocks: blocks,
	}
	err := e.conduit.Unicast(res, originID)
	if err != nil {
		return fmt.Errorf("could not send batch response: %w", err)
	}

	e.metrics.MessageSent(metrics.EngineClusterSynchronization, metrics.MessageBlockResponse)

	return nil
}

// onBlockResponse processes a response containing a specifically requested block.
func (e *Engine) onBlockResponse(originID flow.Identifier, res *messages.ClusterBlockResponse) error {

	// process the blocks one by one
	for _, block := range res.Blocks {
		e.processIncomingBlock(originID, block)
	}

	return nil
}

// processIncoming processes an incoming block, so we can take into account the
// overlap between block IDs and heights.
func (e *Engine) processIncomingBlock(originID flow.Identifier, block *clustermodel.Block) {

	shouldProcess := e.core.HandleBlock(block.Header)
	if !shouldProcess {
		return
	}

	synced := &events.SyncedClusterBlock{
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
		// give the quit channel a priority to be selected
		select {
		case <-e.unit.Quit():
			break CheckLoop
		default:
		}

		select {
		case <-e.unit.Quit():
			break CheckLoop
		case <-poll.C:
			err := e.pollHeight()
			if err != nil {
				e.log.Error().Err(err).Msg("could not poll heights")
			}

		case <-scan.C:
			final, err := e.state.Final().Head()
			if err != nil {
				e.log.Error().Err(err).Msg("could not get final height")
				continue
			}

			e.unit.Lock()
			ranges, batches := e.core.ScanPending(final)
			err = e.sendRequests(ranges, batches)
			if err != nil {
				e.log.Error().Err(err).Msg("could not send requests")
			}
			e.unit.Unlock()
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

	// send the request for synchronization
	req := &messages.SyncRequest{
		Nonce:  rand.Uint64(),
		Height: final.Height,
	}
	err = e.conduit.Multicast(req, synccore.DefaultPollNodes, e.participants.NodeIDs()...)
	if err != nil {
		return fmt.Errorf("could not send sync request: %w", err)
	}
	e.metrics.MessageSent(metrics.EngineClusterSynchronization, metrics.MessageSyncRequest)

	return nil
}

// sendRequests sends a request for each range and batch.
func (e *Engine) sendRequests(ranges []flow.Range, batches []flow.Batch) error {

	var errs error
	for _, ran := range ranges {
		req := &messages.RangeRequest{
			Nonce:      rand.Uint64(),
			FromHeight: ran.From,
			ToHeight:   ran.To,
		}
		err := e.conduit.Multicast(req, synccore.DefaultBlockRequestNodes, e.participants.NodeIDs()...)
		if err != nil {
			errs = multierror.Append(errs, fmt.Errorf("could not submit range request (from=%d, to=%d): %w", ran.From, ran.To, err))
			continue
		}
		e.core.RangeRequested(ran)
		e.metrics.MessageSent(metrics.EngineClusterSynchronization, metrics.MessageRangeRequest)
	}

	for _, batch := range batches {
		req := &messages.BatchRequest{
			Nonce:    rand.Uint64(),
			BlockIDs: batch.BlockIDs,
		}
		err := e.conduit.Multicast(req, synccore.DefaultBlockRequestNodes, e.participants.NodeIDs()...)
		if err != nil {
			errs = multierror.Append(errs, fmt.Errorf("could not submit batch request (size=%d): %w", len(batch.BlockIDs), err))
			continue
		}
		e.core.BatchRequested(batch)
		e.metrics.MessageSent(metrics.EngineClusterSynchronization, metrics.MessageBatchRequest)
	}

	return errs
}
