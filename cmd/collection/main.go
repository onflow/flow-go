package main

import (
	"github.com/spf13/pflag"

	"github.com/dapperlabs/flow-go/cmd"
	"github.com/dapperlabs/flow-go/engine/collection/ingest"
	"github.com/dapperlabs/flow-go/engine/collection/proposal"
	"github.com/dapperlabs/flow-go/engine/collection/provider"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/ingress"
	"github.com/dapperlabs/flow-go/module/mempool"
)

func main() {

	var (
		ingressConfig ingress.Config
		pool          module.TransactionPool
		ingestEngine  *ingest.Engine
		err           error
	)

	cmd.FlowNode("collection").
		Create(func(node *cmd.FlowNodeBuilder) {
			pool, err = mempool.NewTransactionPool()
			node.MustNot(err).Msg("could not initialize transaction pool")
		}).
		ExtraFlags(func(flags *pflag.FlagSet) {
			flags.StringVarP(&ingressConfig.ListenAddr, "ingress-addr", "i", "localhost:9000", "the address the ingress server listens on")
		}).
		Component("ingestion engine", func(node *cmd.FlowNodeBuilder) module.ReadyDoneAware {
			node.Logger.Info().Msg("initializing ingestion engine")

			ingestEngine, err = ingest.New(node.Logger, node.Network, node.State, node.Me, pool)
			node.MustNot(err).Msg("could not initialize ingestion engine")

			return ingestEngine
		}).
		Component("ingress server", func(node *cmd.FlowNodeBuilder) module.ReadyDoneAware {
			node.Logger.Info().Msg("initializing ingress server")

			server := ingress.New(ingressConfig, ingestEngine)
			return server
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
