// Package beaconobservability implements an access node component that walks finalized blocks and
// exports Prometheus metrics about the proposer signature types observed in each block. Under the
// V2 consensus signature scheme, a 48-byte proposer signature indicates a staking-only (fallback)
// proposal, while a 96-byte signature indicates a combined staking + random-beacon share proposal.
// The component also tracks the random beacon threshold and committee membership at epoch boundaries.
//
// The metrics are raw facts only; alerting thresholds and derived ratios are intentionally left to
// Prometheus recording rules.
package beaconobservability

import (
	"fmt"
	"sync"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/jobqueue"
	"github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/state/protocol"
	psEvents "github.com/onflow/flow-go/state/protocol/events"
	"github.com/onflow/flow-go/storage"
)

const (
	// workersCount defines the number of workers that concurrently process finalized block jobs.
	// Sequential processing is required because the metrics depend on block order.
	workersCount = 1

	// searchAhead defines how many blocks the job consumer may look ahead. A value of 1 keeps the
	// consumer strictly sequential.
	searchAhead = 1
)

const (
	// sigTypeStaking is the label value for a bare staking-only proposer signature.
	sigTypeStaking = "staking"
	// sigTypeBeacon is the label value for a combined staking + beacon share proposer signature.
	sigTypeBeacon = "beacon"
)

// BeaconObservability walks finalized blocks from local storage and exports raw facts about the
// proposer signature type observed in each block. It also maintains per-committee-member metric
// series and updates the random beacon threshold at epoch boundaries.
//
// The component is safe for concurrent use. It implements protocol.Consumer so that the protocol
// events distributor can deliver epoch transition notifications.
type BeaconObservability struct {
	*component.ComponentManager
	psEvents.Noop

	log     zerolog.Logger
	state   protocol.State
	blocks  storage.Blocks
	metrics module.BeaconObservabilityMetrics

	finalizedBlockConsumer *jobqueue.ComponentConsumer
	finalizedBlockNotifier engine.Notifier

	epochEvents chan epochTransition

	mu        sync.RWMutex
	committee map[flow.Identifier]struct{}
}

// epochTransition carries the information needed to update committee membership on an epoch change.
type epochTransition struct {
	counter      uint64
	firstBlockID flow.Identifier
}

var _ protocol.Consumer = (*BeaconObservability)(nil)
var _ component.Component = (*BeaconObservability)(nil)

// New creates a new BeaconObservability component.
//
// Expected error returns during normal operation:
//   - [storage.ErrNotFound] if the current finalized state or a required proposal is unavailable
//   - generic error in case of unexpected failure from the protocol state or storage layers
func New(
	log zerolog.Logger,
	state protocol.State,
	blocks storage.Blocks,
	metrics module.BeaconObservabilityMetrics,
	finalizedProcessedHeight storage.ConsumerProgress,
) (*BeaconObservability, error) {
	b := &BeaconObservability{
		log:                    log.With().Str("component", "beacon_observability").Logger(),
		state:                  state,
		blocks:                 blocks,
		metrics:                metrics,
		finalizedBlockNotifier: engine.NewNotifier(),
		epochEvents:            make(chan epochTransition, 1),
		committee:              make(map[flow.Identifier]struct{}),
	}

	if err := b.initializeCommittee(); err != nil {
		return nil, fmt.Errorf("could not initialize committee: %w", err)
	}

	finalizedBlockReader := jobqueue.NewFinalizedBlockReader(state, blocks)
	var err error
	b.finalizedBlockConsumer, err = jobqueue.NewComponentConsumer(
		b.log.With().Str("module", "beacon_observability_block_consumer").Logger(),
		b.finalizedBlockNotifier.Channel(),
		finalizedProcessedHeight,
		finalizedBlockReader,
		b.processFinalizedBlockJob,
		workersCount,
		searchAhead,
	)
	if err != nil {
		return nil, fmt.Errorf("could not create finalized block consumer: %w", err)
	}

	builder := component.NewComponentManagerBuilder().
		AddWorker(b.runFinalizedBlockConsumer).
		AddWorker(b.handleEpochEvents)

	b.ComponentManager = builder.Build()

	return b, nil
}

// BlockFinalized is called by the protocol events distributor when a block is finalized. It notifies
// the internal job queue consumer that a new finalized block is available for processing.
func (b *BeaconObservability) BlockFinalized(h *flow.Header) {
	b.finalizedBlockNotifier.Notify()
}

// EpochTransition is called by the protocol events distributor when the network transitions to a new
// epoch. It queues the event for the internal epoch handling worker.
func (b *BeaconObservability) EpochTransition(newEpochCounter uint64, first *flow.Header) {
	select {
	case b.epochEvents <- epochTransition{counter: newEpochCounter, firstBlockID: first.ID()}:
	default:
		b.log.Warn().
			Uint64("epoch_counter", newEpochCounter).
			Msg("epoch event channel full, dropping epoch transition event")
	}
}

// handleEpochEvents is the worker goroutine that processes epoch transition events. It updates the
// committee membership tracked by the component.
//
// No errors are expected during normal operation.
func (b *BeaconObservability) handleEpochEvents(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	for {
		select {
		case <-ctx.Done():
			return
		case evt := <-b.epochEvents:
			if err := b.updateCommitteeAtBlockID(evt.firstBlockID); err != nil {
				ctx.Throw(fmt.Errorf("could not update committee for epoch %d: %w", evt.counter, err))
				return
			}
		}
	}
}

// runFinalizedBlockConsumer runs the job queue consumer that processes finalized blocks.
//
// No errors are expected during normal operation.
func (b *BeaconObservability) runFinalizedBlockConsumer(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	b.finalizedBlockConsumer.Start(ctx)

	if err := util.WaitClosed(ctx, b.finalizedBlockConsumer.Ready()); err != nil {
		return
	}

	ready()

	<-b.finalizedBlockConsumer.Done()
}

// processFinalizedBlockJob processes a single finalized block job. It reads the corresponding
// proposal from storage, classifies the proposer signature type, and updates the metrics.
//
// Expected error returns during normal operation:
//   - [storage.ErrNotFound] if the proposal for the finalized block is not available in storage
//   - generic error in case of unexpected failure from the storage layer
func (b *BeaconObservability) processFinalizedBlockJob(ctx irrecoverable.SignalerContext, job module.Job, done func()) {
	block, err := jobqueue.JobToBlock(job)
	if err != nil {
		ctx.Throw(fmt.Errorf("failed to convert job to block: %w", err))
		return
	}

	proposal, err := b.blocks.ProposalByHeight(block.Height)
	if err != nil {
		ctx.Throw(fmt.Errorf("could not get proposal by height %d: %w", block.Height, err))
		return
	}

	sigType, err := classifySigType(proposal.ProposerSigData)
	if err != nil {
		b.log.Warn().
			Err(err).
			Uint64("height", block.Height).
			Hex("proposer_id", block.ToHeader().ProposerID[:]).
			Msg("skipping block with non-V2 proposer signature")
		done()
		return
	}

	proposerID := block.ToHeader().ProposerID
	b.metrics.NetworkFinalizedProposerSigType(proposerID, sigType)
	b.metrics.NetworkFinalizedLastProposalHeight(proposerID, block.Height)

	done()
}

// classifySigType classifies the proposer signature data as staking-only or combined based on its
// length. This is the V2 discriminator: a bare BLS staking signature is 48 bytes, while a combined
// staking + beacon share signature is 96 bytes.
//
// Expected error returns during normal operation:
//   - [signature.ErrInvalidSignatureFormat] if the signature data length is neither 48 nor 96 bytes
func classifySigType(sigData []byte) (string, error) {
	switch len(sigData) {
	case signature.SigLen:
		return sigTypeStaking, nil
	case 2 * signature.SigLen:
		return sigTypeBeacon, nil
	default:
		return "", fmt.Errorf("unexpected proposer sig data length %d: %w", len(sigData), signature.ErrInvalidSignatureFormat)
	}
}

// initializeCommittee reads the current consensus committee and DKG information from the finalized
// protocol state and initializes the per-member metric series.
//
// Expected error returns during normal operation:
//   - [storage.ErrNotFound] if the current epoch is not available
//   - generic error in case of unexpected failure from the protocol state or storage layers
func (b *BeaconObservability) initializeCommittee() error {
	return b.updateCommitteeAtSnapshot(b.state.Final())
}

// updateCommitteeAtBlockID reads the consensus committee and DKG information from the protocol state
// at the given block and updates the tracked membership.
//
// Expected error returns during normal operation:
//   - [storage.ErrNotFound] if the epoch at the given block is not available
//   - generic error in case of unexpected failure from the protocol state or storage layers
func (b *BeaconObservability) updateCommitteeAtBlockID(blockID flow.Identifier) error {
	return b.updateCommitteeAtSnapshot(b.state.AtBlockID(blockID))
}

// updateCommitteeAtSnapshot updates the tracked committee membership from the given protocol snapshot.
// It deletes metric series for nodes that left the committee and pre-initializes series for new
// members so that silent nodes remain visible in Prometheus.
//
// Expected error returns during normal operation:
//   - [storage.ErrNotFound] if the current epoch is not available
//   - generic error in case of unexpected failure from the protocol state or storage layers
func (b *BeaconObservability) updateCommitteeAtSnapshot(snapshot protocol.Snapshot) error {
	epoch, err := snapshot.Epochs().Current()
	if err != nil {
		return fmt.Errorf("could not get current epoch: %w", err)
	}

	dkg, err := epoch.DKG()
	if err != nil {
		return fmt.Errorf("could not get dkg: %w", err)
	}

	threshold := uint64(signature.RandomBeaconThreshold(int(dkg.Size())) + 1)
	b.metrics.NetworkDKGBeaconThreshold(threshold)

	identities := epoch.InitialIdentities()
	committeeMembers := identities.Filter(filter.IsConsensusCommitteeMember)

	b.mu.Lock()
	defer b.mu.Unlock()

	newCommittee := make(map[flow.Identifier]struct{}, len(committeeMembers))
	for _, identity := range committeeMembers {
		newCommittee[identity.NodeID] = struct{}{}
	}

	// Delete series for nodes that left the committee.
	for nodeID := range b.committee {
		if _, ok := newCommittee[nodeID]; !ok {
			b.metrics.NetworkFinalizedDeleteProposerMetrics(nodeID)
		}
	}

	// Pre-initialize series for new members so that silent nodes are visible in Prometheus.
	for nodeID := range newCommittee {
		if _, ok := b.committee[nodeID]; !ok {
			b.metrics.NetworkFinalizedInitProposerMetrics(nodeID)
		}
	}

	b.committee = newCommittee
	return nil
}
