package cmd

import (
	"github.com/dapperlabs/flow-go/cmd/bootstrap/run"
	model "github.com/dapperlabs/flow-go/model/bootstrap"
	"github.com/dapperlabs/flow-go/model/flow"
)

func constructRootBlock(nodes []model.NodeInfo, chainID flow.ChainID) *flow.Block {
	identityList := generateIdentityList(nodes)
	block := run.GenerateRootBlock(identityList, chainID)

	writeJSON(model.PathRootBlock, block)

	return block
}

func generateIdentityList(nodes []model.NodeInfo) flow.IdentityList {

	list := make([]*flow.Identity, 0, len(nodes))

	for _, node := range nodes {
		ident := node.Identity()
		list = append(list, ident)
	}

	return list
}
