package main

import (
	"github.com/spf13/pflag"

	"github.com/dapperlabs/flow-go/cmd"
	"github.com/dapperlabs/flow-go/engine/collection/ingest"
	"github.com/dapperlabs/flow-go/engine/collection/proposal"
	"github.com/dapperlabs/flow-go/engine/collection/provider"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/builder/collection"
	"github.com/dapperlabs/flow-go/module/ingress"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/module/mempool/stdmap"
	storage "github.com/dapperlabs/flow-go/storage/badger"
)

func main() {

	var (
		pool        mempool.Transactions
		collections *storage.Collections
		ingressConf ingress.Config
		providerEng *provider.Engine
		ingestEng   *ingest.Engine
		err         error
	)

	cmd.FlowNode("collection").
		Create(func(node *cmd.FlowNodeBuilder) {
			pool, err = stdmap.NewTransactions()
			node.MustNot(err).Msg("could not initialize transaction pool")
		}).
		ExtraFlags(func(flags *pflag.FlagSet) {
			flags.StringVarP(&ingressConf.ListenAddr, "ingress-addr", "i", "localhost:9000", "the address the ingress server listens on")
		}).
		Component("ingestion engine", func(node *cmd.FlowNodeBuilder) module.ReadyDoneAware {
			node.Logger.Info().Msg("initializing ingestion engine")
			ingestEng, err = ingest.New(node.Logger, node.Network, node.State, node.Tracer, node.Me, pool)
			node.MustNot(err).Msg("could not initialize ingestion engine")

			return ingestEng
		}).
		Component("ingress server", func(node *cmd.FlowNodeBuilder) module.ReadyDoneAware {
			node.Logger.Info().Msg("initializing ingress server")

			server := ingress.New(ingressConf, ingestEng)
			return server
		}).
		Component("provider engine", func(node *cmd.FlowNodeBuilder) module.ReadyDoneAware {
			node.Logger.Info().Msg("initializing provider engine")
			collections = storage.NewCollections(node.DB)
			providerEng, err = provider.New(node.Logger, node.Network, node.State, node.Tracer, node.Me, collections)
			node.MustNot(err).Msg("could not initialize proposal engine")
			return providerEng
		}).
		Component("proposal engine", func(node *cmd.FlowNodeBuilder) module.ReadyDoneAware {
			node.Logger.Info().Msg("initializing proposal engine")
			guarantees := storage.NewGuarantees(node.DB)
			headers := storage.NewHeaders(node.DB)
			// TODO determine chain ID for clusters
			build := collection.NewBuilder(node.DB, pool, "TODO")
			// TODO implement finalizer
			var final module.Finalizer
			eng, err := proposal.New(node.Logger, node.Network, node.Me, node.State, node.Tracer, providerEng, pool, collections, guarantees, headers, build, final)
			node.MustNot(err).Msg("could not initialize proposal engine")
			return eng
		}).
		Run()
}
