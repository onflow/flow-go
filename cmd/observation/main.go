package main

import (
	"github.com/spf13/pflag"
	"google.golang.org/grpc"

	"github.com/dapperlabs/flow-go/cmd"
	"github.com/dapperlabs/flow-go/engine/observation/ingestion"
	"github.com/dapperlabs/flow-go/engine/observation/rpc"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/protobuf/services/observation"
	storage "github.com/dapperlabs/flow-go/storage/badger"
)

func main() {

	var (
		blockLimit      uint
		collectionLimit uint
		receiptLimit    uint
		ingestEng       *ingestion.Engine
		rpcConf         rpc.Config
		collectionRPC   observation.ObserveServiceClient
		executionRPC    observation.ObserveServiceClient
		err             error
		blocks          *storage.Blocks
		headers         *storage.Headers
		collections     *storage.Collections
		transactions    *storage.Transactions
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
		Module("persistent storage", func(node *cmd.FlowNodeBuilder) error {
			blocks = storage.NewBlocks(node.DB)
			headers = storage.NewHeaders(node.DB)
			collections = storage.NewCollections(node.DB)
			transactions = storage.NewTransactions(node.DB)
			return nil
		}).
		//Component("follower engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
		//	cache := buffer.NewPendingBlocks()
		//	// using Mock TODO: Create the follower engine here
		//	hsf := new(mock.HotStuffFollower)
		//	// Not sure right now what to put in for
		//	// dkgPubData,  trustedRootBlock and rootBlockSigs in follower_loop follower.New()
		//	followEng, err := follower.New(node.Logger, node.Network, node.Me, node.State, headers, payloads, cache, hsf)
		//	return followEng, err
		//}).
		Component("ingestion engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			ingestEng, err = ingestion.New(node.Logger, node.Network, node.State, node.Tracer, node.Me, blocks, headers, collections, transactions)
			return ingestEng, err
		}).
		Component("RPC engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			rpcEng := rpc.New(node.Logger, node.State, rpcConf, collectionRPC, executionRPC, blocks, headers, collections, transactions)
			return rpcEng, nil
		}).
		Run()
}
