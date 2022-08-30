package consensus

import (
	"github.com/onflow/flow-go/model/events"
	"github.com/onflow/flow-go/model/messages"
)

// ProposalProvider provides proposals created by this node to non-consensus nodes.
type ProposalProvider interface {
	// ProvideProposal asynchronously submits our proposal to all non-consensus nodes.
	ProvideProposal(proposal *messages.BlockProposal)
}

// ComplianceProcessor accepts blocks for ingestion and processing by the compliance layer.
// It is used to ingest blocks received through other means than the HotStuff happy path,
// like the block synchronization engine.
type ComplianceProcessor interface {
	// IngestBlock ingests and queues the block for later processing by the compliance layer.
	IngestBlock(block *events.SyncedBlock)
}
