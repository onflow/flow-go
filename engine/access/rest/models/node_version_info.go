package models

import (
	"github.com/onflow/flow-go/access"
)

func (t *NodeVersionInfo) Build(params *access.NodeVersionInfo) {
	t.Semver = params.Semver
	t.Commit = params.Commit
	t.SporkId = params.SporkId.String()
	t.ProtocolVersion = string(params.ProtocolVersion)
}
