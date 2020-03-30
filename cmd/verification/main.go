package main

import (
	"fmt"

	"github.com/spf13/pflag"

	"github.com/dapperlabs/cadence/runtime"

	"github.com/dapperlabs/flow-go/cmd"
	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/follower"
	followereng "github.com/dapperlabs/flow-go/engine/common/follower"
	"github.com/dapperlabs/flow-go/engine/common/synchronization"
	"github.com/dapperlabs/flow-go/engine/execution/computation/virtualmachine"
	"github.com/dapperlabs/flow-go/engine/verification/ingest"
	"github.com/dapperlabs/flow-go/engine/verification/verifier"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/buffer"
	"github.com/dapperlabs/flow-go/module/chunks"
	finalizer "github.com/dapperlabs/flow-go/module/finalizer/follower"
	"github.com/dapperlabs/flow-go/module/mempool/stdmap"
	storage "github.com/dapperlabs/flow-go/storage/badger"
)

func main() {

	var (
		receiptLimit         uint
		collectionLimit      uint
		blockLimit           uint
		chunkLimit           uint
		err                  error
		authReceipts         *stdmap.Receipts
		pendingReceipts      *stdmap.PendingReceipts
		blockStorage         *storage.Blocks
		headerStorage        *storage.Headers
		conPayloads          *storage.Payloads
		conCache             *buffer.PendingBlocks
		authCollections      *stdmap.Collections
		pendingCollections   *stdmap.PendingCollections
		collectionTrackers   *stdmap.CollectionTrackers
		chunkStates          *stdmap.ChunkStates
		chunkDataPacks       *stdmap.ChunkDataPacks
		chunkDataPackTracker *stdmap.ChunkDataPackTrackers
		chunkStateTracker    *stdmap.ChunkStateTrackers
		verifierEng          *verifier.Engine
		ingestEng            *ingest.Engine
	)

	cmd.FlowNode("verification").
		ExtraFlags(func(flags *pflag.FlagSet) {
			flags.UintVar(&receiptLimit, "receipt-limit", 100000, "maximum number of execution receipts in the memory pool")
			flags.UintVar(&collectionLimit, "collection-limit", 100000, "maximum number of authCollections in the memory pool")
			flags.UintVar(&blockLimit, "block-limit", 100000, "maximum number of result blocks in the memory pool")
			flags.UintVar(&chunkLimit, "chunk-limit", 100000, "maximum number of chunk states in the memory pool")
		}).
		Module("execution authenticated receipts mempool", func(node *cmd.FlowNodeBuilder) error {
			authReceipts, err = stdmap.NewReceipts(receiptLimit)
			return err
		}).
		Module("execution pending receipts mempool", func(node *cmd.FlowNodeBuilder) error {
			pendingReceipts, err = stdmap.NewPendingReceipts(receiptLimit)
			return err
		}).
		Module("authenticated collections mempool", func(node *cmd.FlowNodeBuilder) error {
			authCollections, err = stdmap.NewCollections(collectionLimit)
			return err
		}).
		Module("pending collections mempool", func(node *cmd.FlowNodeBuilder) error {
			pendingCollections, err = stdmap.NewPendingCollections(collectionLimit)
			return err
		}).
		Module("collection trackers mempool", func(node *cmd.FlowNodeBuilder) error {
			collectionTrackers, err = stdmap.NewCollectionTrackers(collectionLimit)
			return err
		}).
		Module("persistent storage", func(node *cmd.FlowNodeBuilder) error {
			// creates a block storage for the node
			// to reflect incoming blocks on state
			blockStorage = storage.NewBlocks(node.DB)
			// headers and consensus storage for consensus follower engine
			headerStorage = storage.NewHeaders(node.DB)
			conPayloads = storage.NewPayloads(node.DB)
			return nil
		}).
		Module("chunk states mempool", func(node *cmd.FlowNodeBuilder) error {
			chunkStates, err = stdmap.NewChunkStates(chunkLimit)
			return err
		}).
		Module("chunk state tracker mempool", func(node *cmd.FlowNodeBuilder) error {
			chunkStateTracker, err = stdmap.NewChunkStateTrackers(chunkLimit)
			return err
		}).
		Module("chunk data pack mempool", func(node *cmd.FlowNodeBuilder) error {
			chunkDataPacks, err = stdmap.NewChunkDataPacks(chunkLimit)
			return err
		}).
		Module("chunk data pack tracker mempool", func(node *cmd.FlowNodeBuilder) error {
			chunkDataPackTracker, err = stdmap.NewChunkDataPackTrackers(chunkLimit)
			return err
		}).
		Module("block cache", func(node *cmd.FlowNodeBuilder) error {
			// consensus cache for follower engine
			conCache = buffer.NewPendingBlocks()
			return nil
		}).
		Component("verifier engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			rt := runtime.NewInterpreterRuntime()
			vm := virtualmachine.New(rt)
			chunkVerifier := chunks.NewChunkVerifier(vm)
			verifierEng, err = verifier.New(node.Logger, node.Network, node.State, node.Me, chunkVerifier)
			return verifierEng, err
		}).
		Component("ingest engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			alpha := 10
			assigner, err := chunks.NewPublicAssignment(alpha)
			if err != nil {
				return nil, err
			}
			// https://github.com/dapperlabs/flow-go/issues/2703
			// proper place and only referenced here
			// Todo the hardcoded default value should be parameterized as alpha in a
			// should be moved to a configuration class
			// DISCLAIMER: alpha down there is not a production-level value

			ingestEng, err = ingest.New(node.Logger,
				node.Network,
				node.State,
				node.Me,
				verifierEng,
				authReceipts,
				pendingReceipts,
				authCollections,
				pendingCollections,
				collectionTrackers,
				chunkStates,
				chunkStateTracker,
				chunkDataPacks,
				chunkDataPackTracker,
				blockStorage,
				assigner)
			return ingestEng, err
		}).
		Component("follower engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			// create a finalizer that handles updating the protocol
			// state when the follower detects newly finalized blocks
			final := finalizer.NewFinalizer(node.DB)

			// TODO: initialize hotstuff verifier
			var verifier hotstuff.Verifier

			// creates a consensus follower with ingestEngine as the notifier
			// so that it gets notified upon each new finalized block
			core, err := follower.New(node.Me,
				node.State,
				node.DKGState,
				&node.GenesisBlock.Header,
				node.GenesisQC,
				verifier,
				final,
				ingestEng,
				node.Logger)
			if err != nil {
				// return nil, fmt.Errorf("could not create follower core logic: %w", err)
				// TODO for now we ignore failures in follower
				// this is necessary for integration tests to run, until they are
				// updated to generate/use valid genesis QC and DKG files.
				// ref https://github.com/dapperlabs/flow-go/issues/3057
				node.Logger.Debug().Err(err).Msg("ignoring failures in follower core")
			}

			followerEng, err := followereng.New(node.Logger,
				node.Network,
				node.Me,
				node.State,
				headerStorage,
				conPayloads,
				conCache,
				core)
			if err != nil {
				return nil, fmt.Errorf("could not create follower engine: %w", err)
			}

			// create a block synchronization engine to handle follower getting
			// out of sync
			sync, err := synchronization.New(node.Logger,
				node.Network,
				node.Me,
				node.State,
				blockStorage,
				followerEng)
			if err != nil {
				return nil, fmt.Errorf("could not create synchronization engine: %w", err)
			}

			return followerEng.WithSynchronization(sync), nil
		}).
		Run()
}
