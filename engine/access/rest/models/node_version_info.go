package models

import (
	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/util"
)

func (t *NodeVersionInfo) Build(params *access.NodeVersionInfo) {
	t.Semver = params.Semver
	t.Commit = params.Commit
	t.SporkId = params.SporkId.String()
	t.ProtocolVersion = util.FromUint64(params.ProtocolVersion)
	t.SporkRootBlockHeight = util.FromUint64(params.SporkRootBlockHeight)
	t.NodeRootBlockHeight = util.FromUint64(params.NodeRootBlockHeight)
}
