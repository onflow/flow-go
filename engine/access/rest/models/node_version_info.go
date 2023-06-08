package models

import (
	"encoding/hex"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/util"
	"github.com/onflow/flow/protobuf/go/flow/entities"
)

func (t *NodeVersionInfo) Build(params *access.NodeVersionInfo) {
	t.Semver = params.Semver
	t.Commit = params.Commit
	t.SporkId = params.SporkId.String()
	t.ProtocolVersion = util.FromUint64(params.ProtocolVersion)
}

func (t *NodeVersionInfo) BuildFromGrpc(params *entities.NodeVersionInfo) {
	t.Semver = params.Semver
	t.Commit = params.Commit
	t.SporkId = hex.EncodeToString(params.SporkId)
	t.ProtocolVersion = util.FromUint64(params.ProtocolVersion)
}
