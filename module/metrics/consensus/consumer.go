package consensus

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/notifications"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/storage"
)

// MetricsConsumer is a consumer that subscribes to hotstuff events and
// collects metrics data when certain events trigger.
// It depends on Metrics module to report metrics data.
type MetricsConsumer struct {
	// inherit from noop consumer in order to satisfy the full interface
	notifications.NoopConsumer
	log        zerolog.Logger
	guarantees storage.Guarantees
	seals      storage.Seals
	metrics    module.Metrics
}

func NewMetricsConsumer(metrics module.Metrics, guarantees storage.Guarantees, seals storage.Seals) *MetricsConsumer {
	return &MetricsConsumer{
		metrics:    metrics,
		guarantees: guarantees,
		seals:      seals,
	}
}

func (c *MetricsConsumer) OnFinalizedBlock(block *model.Block) {
	c.metrics.FinalizedBlocks(1)
	c.metrics.FinalizedBlockView(block.View)

	err := c.traceFinalizedCollections(block)
	if err != nil {
		c.log.Err(fmt.Errorf("could not trace collection when block finalized: %w", err))
	}

	err = c.traceFinalizedSeals(block)
	if err != nil {
		c.log.Err(fmt.Errorf("could not trace seals when block finalized: %w", err))
	}
}

func (c *MetricsConsumer) OnBlockIncorporated(block *model.Block) {
	guarantees, err := c.guarantees.ByBlockID(block.BlockID)
	if err != nil {
		c.log.Err(fmt.Errorf("could not get guarantee: %w", err))
		return
	}

	// monitor the number of collections included per incorporated block
	c.metrics.CollectionsPerBlock(len(guarantees))
}

func (c *MetricsConsumer) OnEnteringView(view uint64) {
	c.metrics.StartNewView(view)
}

func (c *MetricsConsumer) OnForkChoiceGenerated(uint64, *model.QuorumCertificate) {
	c.metrics.MadeBlockProposal()
}

func (c *MetricsConsumer) OnQcIncorporated(qc *model.QuorumCertificate) {
	c.metrics.NewestKnownQC(qc.View)
}

// trace the end of the duration from when a collection is received to when it's finalized
func (c *MetricsConsumer) traceFinalizedCollections(block *model.Block) error {
	collections, err := c.guarantees.ByBlockID(block.BlockID)
	if err != nil {
		return fmt.Errorf("could not get guarantee: %w", err)
	}

	// trace collection duration
	// reports Metrics C1: Collection Received by CCL→ Collection Included in Finalized Block
	for _, collection := range collections {
		c.metrics.FinishCollectionToFinalized(collection.ID())
	}

	// collection included in finalized blocks
	c.metrics.CollectionsInFinalizedBlock(len(collections))
	return nil
}

// trace the end of duration from when a block is received to when it's sealed
func (c *MetricsConsumer) traceFinalizedSeals(block *model.Block) error {
	seals, err := c.seals.ByBlockID(block.BlockID)
	if err != nil {
		return fmt.Errorf("could not get seals: %w", err)
	}
	// trace seal duration
	for _, seal := range seals {
		c.metrics.FinishBlockToSeal(seal.BlockID)
	}

	// report number of seals included in finalized blocks
	c.metrics.SealsInFinalizedBlock(len(seals))

	return nil
}
