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
		blockLimit      uint
		collectionLimit uint
		receiptLimit    uint
		blockPool       mempool.Blocks
		collPool        mempool.Collections
		receiptPool     mempool.Receipts
		ingestEng       *ingestion.Engine
		rpcConf         rpc.Config
		collectionRPC   observation.ObserveServiceClient
		executionRPC    observation.ObserveServiceClient
		err             error
	)

	cmd.FlowNode("observation").
		ExtraFlags(func(flags *pflag.FlagSet) {
			flags.UintVar(&receiptLimit, "receipt-limit", 100000, "maximum number of execution receipts in the memory pool")
			flags.UintVar(&collectionLimit, "collection-limit", 100000, "maximum number of collections in the memory pool")
			flags.UintVar(&blockLimit, "block-limit", 100000, "maximum number of result blocks in the memory pool")
			flags.StringVarP(&rpcConf.ListenAddr, "rpc-addr", "i", "localhost:9000", "the address the gRPC server listens on")
			flags.StringVarP(&rpcConf.CollectionAddr, "ingress-addr", "i", "localhost:9000", "the address (of the collection node) to send transactions to")
			flags.StringVarP(&rpcConf.ExecutionAddr, "script-addr", "i", "localhost:9000", "the address (of the execution node) forward the script to")
		}).
		Module("blocks mempool", func(node *cmd.FlowNodeBuilder) error {
			blockPool, err = stdmap.NewBlocks(blockLimit)
			return err
		}).
		Module("collections mempool", func(node *cmd.FlowNodeBuilder) error {
			collPool, err = stdmap.NewCollections(collectionLimit)
			return err
		}).
		Module("execution receipts mempool", func(node *cmd.FlowNodeBuilder) error {
			receiptPool, err = stdmap.NewReceipts(receiptLimit)
			return err
		}).
		Module("collection node client", func(node *cmd.FlowNodeBuilder) error {
			collectionRPCConn, err := grpc.Dial(rpcConf.CollectionAddr)
			if err != nil {
				return err
			}
			collectionRPC = observation.NewObserveServiceClient(collectionRPCConn)
			return nil
		}).
		Module("execution node client", func(node *cmd.FlowNodeBuilder) error {
			executionRPCConn, err := grpc.Dial(rpcConf.ExecutionAddr)
			if err != nil {
				return err
			}
			executionRPC = observation.NewObserveServiceClient(executionRPCConn)
			return nil
		}).
		Component("ingestion engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			ingestEng, err = ingestion.New(node.Logger, node.Network, node.State, node.Tracer, node.Me, blockPool, collPool, receiptPool)
			return ingestEng, err
		}).
		Component("RPC engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			rpcEng := rpc.New(node.Logger, rpcConf, collectionRPC, executionRPC)
			return rpcEng, nil
		}).
		Run()
}
