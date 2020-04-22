package main

import (
	"fmt"

	"github.com/spf13/pflag"

	"github.com/onflow/cadence/runtime"

	"github.com/dapperlabs/flow-go/cmd"
	"github.com/dapperlabs/flow-go/consensus"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/verification"
	followereng "github.com/dapperlabs/flow-go/engine/common/follower"
	"github.com/dapperlabs/flow-go/engine/common/synchronization"
	"github.com/dapperlabs/flow-go/engine/execution/computation/virtualmachine"
	"github.com/dapperlabs/flow-go/engine/verification/ingest"
	"github.com/dapperlabs/flow-go/engine/verification/verifier"
	"github.com/dapperlabs/flow-go/model/encoding"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/buffer"
	"github.com/dapperlabs/flow-go/module/chunks"
	finalizer "github.com/dapperlabs/flow-go/module/finalizer/follower"
	"github.com/dapperlabs/flow-go/module/mempool/stdmap"
	"github.com/dapperlabs/flow-go/module/signature"
	storage "github.com/dapperlabs/flow-go/storage/badger"
)

const (
	// following lists the operational parameters of verification node
	//
	// chunkAssignmentAlpha represents number of verification
	// DISCLAIMER: alpha down there is not a production-level value
	chunkAssignmentAlpha = 1
)

func main() {

	var (
		alpha                uint
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
		chunkDataPacks       *stdmap.ChunkDataPacks
		chunkDataPackTracker *stdmap.ChunkDataPackTrackers
		ingestedChunkIDs     *stdmap.Identifiers
		ingestedResultIDs    *stdmap.Identifiers
		verifierEng          *verifier.Engine
		ingestEng            *ingest.Engine
	)

	cmd.FlowNode("verification").
		ExtraFlags(func(flags *pflag.FlagSet) {
			flags.UintVar(&receiptLimit, "receipt-limit", 100000, "maximum number of execution receipts in the memory pool")
			flags.UintVar(&collectionLimit, "collection-limit", 100000, "maximum number of authCollections in the memory pool")
			flags.UintVar(&blockLimit, "block-limit", 100000, "maximum number of result blocks in the memory pool")
			flags.UintVar(&chunkLimit, "chunk-limit", 100000, "maximum number of chunk states in the memory pool")
			flags.UintVar(&alpha, "alpha", 10, "maximum number of chunk states in the memory pool")
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
		Module("chunk data pack mempool", func(node *cmd.FlowNodeBuilder) error {
			chunkDataPacks, err = stdmap.NewChunkDataPacks(chunkLimit)
			return err
		}).
		Module("chunk data pack tracker mempool", func(node *cmd.FlowNodeBuilder) error {
			chunkDataPackTracker, err = stdmap.NewChunkDataPackTrackers(chunkLimit)
			return err
		}).
		Module("ingested chunk ids mempool", func(node *cmd.FlowNodeBuilder) error {
			ingestedChunkIDs, err = stdmap.NewIdentifiers(chunkLimit)
			return err
		}).
		Module("ingested result ids mempool", func(node *cmd.FlowNodeBuilder) error {
			ingestedResultIDs, err = stdmap.NewIdentifiers(receiptLimit)
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
			verifierEng, err = verifier.New(node.Logger, node.Network, node.State, node.Me, chunkVerifier, node.Metrics)
			return verifierEng, err
		}).
		Component("ingest engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {

			assigner, err := chunks.NewPublicAssignment(chunkAssignmentAlpha)
			if err != nil {
				return nil, err
			}
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
				chunkDataPacks,
				chunkDataPackTracker,
				ingestedChunkIDs,
				ingestedResultIDs,
				blockStorage,
				assigner)
			return ingestEng, err
		}).
		Component("follower engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			// create a finalizer that handles updating the protocol
			// state when the follower detects newly finalized blocks
			final := finalizer.NewFinalizer(node.DB)

			// initialize the staking & beacon verifiers, signature joiner
			staking := signature.NewAggregationVerifier(encoding.ConsensusVoteTag)
			beacon := signature.NewThresholdVerifier(encoding.RandomBeaconTag)
			merger := signature.NewCombiner()

			// define the node set that is valid as signers
			selector := filter.And(filter.HasRole(flow.RoleConsensus), filter.HasStake(true))

			// initialize the verifier for the protocol consensus
			verifier := verification.NewCombinedVerifier(node.State, node.DKGState, staking, beacon, merger, selector)

			// creates a consensus follower with ingestEngine as the notifier
			// so that it gets notified upon each new finalized block
			core, err := consensus.NewFollower(node.Logger, node.State, node.Me, final, verifier, ingestEng, &node.GenesisBlock.Header, node.GenesisQC, selector)
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
		Run("verification")
}
