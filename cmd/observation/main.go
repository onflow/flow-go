package main

import (
	"fmt"

	"github.com/spf13/pflag"
	"google.golang.org/grpc"

	"github.com/dapperlabs/flow-go/cmd"
	"github.com/dapperlabs/flow-go/module/buffer"

	"github.com/dapperlabs/flow/protobuf/go/flow/access"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/follower"
	followereng "github.com/dapperlabs/flow-go/engine/common/follower"
	"github.com/dapperlabs/flow-go/engine/common/synchronization"
	"github.com/dapperlabs/flow-go/engine/observation/ingestion"
	"github.com/dapperlabs/flow-go/engine/observation/rpc"
	"github.com/dapperlabs/flow-go/module"
	followerfinalizer "github.com/dapperlabs/flow-go/module/finalizer/follower"
	storage "github.com/dapperlabs/flow-go/storage/badger"
)

func main() {

	var (
		blockLimit      uint
		collectionLimit uint
		receiptLimit    uint
		ingestEng       *ingestion.Engine
		rpcConf         rpc.Config
		collectionRPC   access.AccessAPIClient
		executionRPC    access.AccessAPIClient
		err             error
		blocks          *storage.Blocks
		headers         *storage.Headers
		payloads        *storage.Payloads
		collections     *storage.Collections
		transactions    *storage.Transactions
		conCache        *buffer.PendingBlocks // pending block cache for follower
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
			collectionRPC = access.NewAccessAPIClient(collectionRPCConn)
			return nil
		}).
		Module("execution node client", func(node *cmd.FlowNodeBuilder) error {
			executionRPCConn, err := grpc.Dial(rpcConf.ExecutionAddr)
			if err != nil {
				return err
			}
			executionRPC = access.NewAccessAPIClient(executionRPCConn)
			return nil
		}).
		Module("persistent storage", func(node *cmd.FlowNodeBuilder) error {
			blocks = storage.NewBlocks(node.DB)
			headers = storage.NewHeaders(node.DB)
			collections = storage.NewCollections(node.DB)
			transactions = storage.NewTransactions(node.DB)
			return nil
		}).
		Module("block cache", func(node *cmd.FlowNodeBuilder) error {
			conCache = buffer.NewPendingBlocks()
			return nil
		}).
		Component("ingestion engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			ingestEng, err = ingestion.New(node.Logger, node.Network, node.State, node.Tracer, node.Me, blocks, headers, collections, transactions)
			return ingestEng, err
		}).
		Component("follower engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			// create a finalizer that will handle updating the protocol
			// state when the follower detects newly finalized blocks
			final := followerfinalizer.NewFinalizer(node.DB)

			// create a follower with the ingestEng as the finalization notification consumer
			core, err := follower.New(node.Me, node.State, node.DKGPubData, &node.GenesisBlock.Header, node.GenesisQC, final, ingestEng, node.Logger)
			if err != nil {
				// TODO for now we ignore failures in follower
				// this is necessary for integration tests to run, until they are
				// updated to generate/use valid genesis QC and DKG files.
				// ref https://github.com/dapperlabs/flow-go/issues/3057
				node.Logger.Debug().Err(err).Msg("ignoring failures in follower core")
			}

			follower, err := followereng.New(node.Logger, node.Network, node.Me, node.State, headers, payloads, conCache, core)
			if err != nil {
				return nil, fmt.Errorf("could not create follower engine: %w", err)
			}

			// create a block synchronization engine to handle follower getting
			// out of sync
			sync, err := synchronization.New(node.Logger, node.Network, node.Me, node.State, blocks, follower)
			if err != nil {
				return nil, fmt.Errorf("could not create synchronization engine: %w", err)
			}

			return follower.WithSynchronization(sync), nil
		}).
		Component("RPC engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			rpcEng := rpc.New(node.Logger, node.State, rpcConf, collectionRPC, executionRPC, blocks, headers, collections, transactions)
			return rpcEng, nil
		}).
		Run()
}
