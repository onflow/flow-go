package run

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol/inmem"
)

// GenerateRootProtocolSnapshot generates the root protocol snapshot to be used
// for bootstrapping a network
func GenerateRootProtocolSnapshot(block *flow.Block, rootQC *flow.QuorumCertificate,
	result *flow.ExecutionResult, seal *flow.Seal) (*inmem.Snapshot, error) {

	return nil, nil
}
