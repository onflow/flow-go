package models

import (
	"github.com/onflow/flow-go/engine/access/rest/util"
	accessmodel "github.com/onflow/flow-go/model/access"
)

func (t *NodeVersionInfo) Build(params *accessmodel.NodeVersionInfo) {
	t.Semver = params.Semver
	t.Commit = params.Commit
	t.SporkId = params.SporkId.String()
	t.ProtocolStateVersion = util.FromUint(params.ProtocolStateVersion)
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
