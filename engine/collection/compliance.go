package collection

import (
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
)

// Compliance defines the interface to the cluster consensus logic that precedes hotstuff logic.
// It's responsible for processing incoming block proposals broadcast by other cluster consensus participants
// as well as blocks obtained via the chain sync protocol.
// Compliance logic performs validation of incoming blocks by checking headers and payloads.
// Compliance logic guarantees that only valid blocks are added to chain state, passed to hotstuff and other
// components.
// Implementation need to be non-blocking and concurrency safe.
type Compliance interface {
	component.Component

	// OnClusterBlockProposal feeds a new block proposal into the processing pipeline.
	// Inputs are structurally valid, but untrusted and must be validated by internal business logic.
	// Incoming proposals will be queued and eventually dispatched by worker.
	// This method is non-blocking.
	OnClusterBlockProposal(proposal flow.Slashable[*cluster.Proposal])
	// OnSyncedClusterBlock feeds a new block proposal into the processing pipeline.
	// Inputs are structurally valid, but untrusted and must be validated by internal business logic.
	// Incoming proposals will be queued and eventually dispatched by worker.
	// This method is non-blocking.
	OnSyncedClusterBlock(block flow.Slashable[*cluster.Proposal])
}
