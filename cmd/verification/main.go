package main

import (
	"github.com/dapperlabs/flow-go/cmd"
	"github.com/dapperlabs/flow-go/engine/verification/verifier"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/module/mempool/stdmap"
)

func main() {

	var (
		err         error
		receipts    mempool.Receipts
		blocks      mempool.Blocks
		collections mempool.Collections
	)

	cmd.FlowNode("verification").
		Create(func(node *cmd.FlowNodeBuilder) {
			receipts, err = stdmap.NewReceipts()
			node.MustNot(err).Msg("could not initialize execution receipts mempool")

			collections, err = stdmap.NewCollections()
			node.MustNot(err).Msg("could not initialize result approvals mempool")

			blocks, err = stdmap.NewBlocks()
			node.MustNot(err).Msg("could not initialize blocks mempool")
		}).
		Component("verifier engine", func(node *cmd.FlowNodeBuilder) module.ReadyDoneAware {
			node.Logger.Info().Msg("initializing verifier engine")

			vrfy, err := verifier.New(node.Logger, node.Network, node.State, node.Me, receipts, blocks, collections)
			node.MustNot(err).Msg("could not initialize verifier engine")
			return vrfy
		}).
		Run()
}
