package main

import (
	"github.com/spf13/pflag"
	"google.golang.org/grpc"

	"github.com/dapperlabs/flow-go/cmd"
	"github.com/dapperlabs/flow-go/engine/observation/ingestion"
	"github.com/dapperlabs/flow-go/engine/observation/rpc"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/module/mempool/stdmap"
	"github.com/dapperlabs/flow-go/protobuf/services/observation"
)

func main() {

	var (
		blockPool   mempool.Blocks
		collPool    mempool.Collections
		receiptPool mempool.Receipts
		ingestEng   *ingestion.Engine
		rpcConf     rpc.Config
		err         error
	)

	cmd.FlowNode("observation").
		Create(func(node *cmd.FlowNodeBuilder) {
			collPool, err = stdmap.NewCollections()
			node.MustNot(err).Msg("could not initialize collection pool")
			blockPool, err = stdmap.NewBlocks()
			node.MustNot(err).Msg("could not initialize block pool")
			receiptPool, err = stdmap.NewReceipts()
			node.MustNot(err).Msg("could not initialize receipt pool")
		}).
		ExtraFlags(func(flags *pflag.FlagSet) {
			flags.StringVarP(&rpcConf.ListenAddr, "rpc-addr", "i", "localhost:9000", "the address the gRPC server listens on")
			flags.StringVarP(&rpcConf.CollectionAddr, "ingress-addr", "i", "localhost:9000", "the address (of the collection node) to send transactions to")
			flags.StringVarP(&rpcConf.ExecutionAddr, "script-addr", "i", "localhost:9000", "the address (of the execution node) forward the script to")
		}).
		Component("ingestion engine", func(node *cmd.FlowNodeBuilder) module.ReadyDoneAware {
			node.Logger.Info().Msg("initializing ingestion engine")
			ingestEng, err = ingestion.New(node.Logger, node.Network, node.State, node.Tracer, node.Me, blockPool, collPool, receiptPool)
			node.MustNot(err).Msg("could not initialize ingestion engine")

			return ingestEng
		}).
		Component("RPC engine", func(node *cmd.FlowNodeBuilder) module.ReadyDoneAware {
			node.Logger.Info().Msg("initializing gRPC server")

			collectionRPCConn, err := grpc.Dial(rpcConf.CollectionAddr)
			node.MustNot(err).Msg("could not connect to collection node")
			collectionRPC := observation.NewObserveServiceClient(collectionRPCConn)

			executionRPCConn, err := grpc.Dial(rpcConf.ExecutionAddr)
			node.MustNot(err).Msg("could not connect to execution node")
			executionRPC := observation.NewObserveServiceClient(executionRPCConn)

			rpcEng := rpc.New(node.Logger, rpcConf, ingestEng, collectionRPC, executionRPC)
			return rpcEng
		}).
		Run()
}
