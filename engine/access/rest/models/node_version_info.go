package models

import (
	"github.com/onflow/flow-go/engine/access/rest/util"
	"github.com/onflow/flow-go/model/flow"
)

func (t *NodeVersionInfo) Build(params *flow.NodeVersionInfo) {
	t.Semver = params.Semver
	t.Commit = params.Commit
	t.SporkId = params.SporkId.String()
	t.ProtocolVersion = util.FromUint(params.ProtocolVersion)
	t.SporkRootBlockHeight = util.FromUint(params.SporkRootBlockHeight)
	t.NodeRootBlockHeight = util.FromUint(params.NodeRootBlockHeight)

	if params.CompatibleRange != nil {
		t.CompatibleRange = &CompatibleRange{
			StartHeight: util.FromUint(params.CompatibleRange.StartHeight),
			EndHeight:   util.FromUint(params.CompatibleRange.EndHeight),
		}
	}
}
