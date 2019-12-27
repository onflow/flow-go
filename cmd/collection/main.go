package main

import (
	"github.com/dapperlabs/flow-go/cmd"
	"github.com/dapperlabs/flow-go/engine/collection/ingest"
	"github.com/dapperlabs/flow-go/engine/collection/proposal"
	"github.com/dapperlabs/flow-go/engine/collection/provider"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/mempool"
)

func main() {

	var (
		pool module.TransactionPool
		err  error
	)

	cmd.FlowNode("collection").
		Create(func(node *cmd.FlowNodeBuilder) {
			pool, err = mempool.NewTransactionPool()
			node.MustNot(err).Msg("could not initialize transaction pool")
		}).
		Component("ingestion engine", func(node *cmd.FlowNodeBuilder) module.ReadyDoneAware {
			node.Logger.Info().Msg("initializing ingestion engine")

			eng, err := ingest.New(node.Logger, node.Network, node.State, node.Me, pool)
			node.MustNot(err).Msg("could not initialize ingestion engine")
			return eng
		}).
		Component("proposal engine", func(node *cmd.FlowNodeBuilder) module.ReadyDoneAware {
			node.Logger.Info().Msg("initializing proposal engine")

			eng, err := proposal.New(node.Logger, node.Network, node.State, node.Me)
			node.MustNot(err).Msg("could not initialize proposal engine")
			return eng
		}).
		Component("provider engine", func(node *cmd.FlowNodeBuilder) module.ReadyDoneAware {
			node.Logger.Info().Msg("initializing provider engine")

			eng, err := provider.New(node.Logger, node.Network, node.State, node.Me)
			node.MustNot(err).Msg("could not initialize proposal engine")
			return eng
		}).
		Run()
}
