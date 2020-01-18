package main

import (
	"github.com/dapperlabs/flow-go/cmd"
	"github.com/dapperlabs/flow-go/engine/verification/ingest"
	"github.com/dapperlabs/flow-go/engine/verification/verifier"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/mempool/stdmap"
)

func main() {

	var (
		err         error
		receipts    *stdmap.Receipts
		blocks      *stdmap.Blocks
		collections *stdmap.Collections
		verifierEng *verifier.Engine
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

			verifierEng, err = verifier.New(node.Logger, node.Network, node.State, node.Me)
			node.MustNot(err).Msg("could not initialize verifier engine")
			return verifierEng
		}).
		Component("ingest engine", func(node *cmd.FlowNodeBuilder) module.ReadyDoneAware {
			node.Logger.Info().Msg("initializing ingest engine")

			eng, err := ingest.New(node.Logger, node.Network, node.State, node.Me, verifierEng, receipts, blocks, collections)
			node.MustNot(err).Msg("could not initialize ingest engine")
			return eng
		}).
		Run()
}
