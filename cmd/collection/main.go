package main

import (
	"fmt"
	"time"

	"github.com/spf13/pflag"

	"github.com/dapperlabs/flow-go/cmd"
	"github.com/dapperlabs/flow-go/consensus/coldstuff"
	"github.com/dapperlabs/flow-go/engine/collection/ingest"
	"github.com/dapperlabs/flow-go/engine/collection/proposal"
	"github.com/dapperlabs/flow-go/engine/collection/provider"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/buffer"
	builder "github.com/dapperlabs/flow-go/module/builder/collection"
	finalizer "github.com/dapperlabs/flow-go/module/finalizer/collection"
	"github.com/dapperlabs/flow-go/module/ingress"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/module/mempool/stdmap"
	storage "github.com/dapperlabs/flow-go/storage/badger"
)

func main() {

	var (
		txLimit     uint
		pool        mempool.Transactions
		collections *storage.Collections
		ingressConf ingress.Config
		prov        *provider.Engine
		ing         *ingest.Engine
		err         error
	)

	cmd.FlowNode("collection").
		ExtraFlags(func(flags *pflag.FlagSet) {
			flags.UintVar(&txLimit, "tx-limit", 100000, "maximum number of transactions in the memory pool")
			flags.StringVarP(&ingressConf.ListenAddr, "ingress-addr", "i", "localhost:9000", "the address the ingress server listens on")
		}).
		Module("transactions mempool", func(node *cmd.FlowNodeBuilder) error {
			pool, err = stdmap.NewTransactions(txLimit)
			return err
		}).
		Component("ingestion engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			ing, err = ingest.New(node.Logger, node.Network, node.State, node.Tracer, node.Me, pool)
			return ing, err
		}).
		Component("ingress server", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			server := ingress.New(ingressConf, ing)
			return server, nil
		}).
		Component("provider engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			collections = storage.NewCollections(node.DB)
			prov, err = provider.New(node.Logger, node.Network, node.State, node.Tracer, node.Me, collections)
			return prov, err
		}).
		Component("proposal engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			transactions := storage.NewTransactions(node.DB)
			headers := storage.NewHeaders(node.DB)
			payloads := storage.NewClusterPayloads(node.DB)
			cache := buffer.NewPendingClusterBlocks()
			prop, err := proposal.New(node.Logger, node.Network, node.Me, node.State, node.Tracer, prov, pool, transactions, headers, payloads, cache)
			if err != nil {
				return nil, fmt.Errorf("could not initialize engine: %w", err)
			}

			build := builder.NewBuilder(node.DB, pool, "TODO")
			final := finalizer.NewFinalizer(node.DB, pool, prov, node.Tracer, "TODO")

			clusters, err := node.State.Final().Clusters()
			if err != nil {
				return nil, fmt.Errorf("could not get clusters: %w", err)
			}
			cluster, err := clusters.ByNodeID(node.Me.NodeID())
			if err != nil {
				return nil, fmt.Errorf("could not get my cluster: %w", err)
			}
			participants, err := node.State.Final().Identities(filter.In(cluster))
			if err != nil {
				return nil, fmt.Errorf("could not get nodes in cluster: %w", err)
			}

			cold, err := coldstuff.New(node.Logger, node.State, node.Me, prop, build, final, participants, 3*time.Second, 6*time.Second)
			if err != nil {
				return nil, fmt.Errorf("could not initialize algorithm: %w", err)
			}

			prop = prop.WithConsensus(cold)
			return prop, nil
		}).
		Run()
}
