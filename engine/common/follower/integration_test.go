package follower

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/cockroachdb/pebble/v2"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/consensus"
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/compliance"
	moduleconsensus "github.com/onflow/flow-go/module/finalizer/consensus"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	module "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/module/trace"
	moduleutil "github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/network/mocknetwork"
	pbadger "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/events"
	"github.com/onflow/flow-go/state/protocol/util"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestFollowerHappyPath tests ComplianceEngine integrated with real modules, mocked modules are used only for functionality which is static
// or implemented by our test case. Tests that syncing batches of blocks from other participants results in extending protocol state.
// After processing all available blocks we check if chain has correct height and finalized block.
// We use the following setup:
// Number of workers - workers
// Number of batches submitted by worker - batchesPerWorker
// Number of blocks in each batch submitted by worker - blocksPerBatch
// Each worker submits batchesPerWorker*blocksPerBatch blocks
// In total we will submit workers*batchesPerWorker*blocksPerBatch
func TestFollowerHappyPath(t *testing.T) {
	allIdentities := unittest.CompleteIdentitySet()
	rootSnapshot := unittest.RootSnapshotFixture(allIdentities)
	lockManager := storage.NewTestingLockManager()
	unittest.RunWithPebbleDB(t, func(pdb *pebble.DB) {
		metrics := metrics.NewNoopCollector()
		tracer := trace.NewNoopTracer()
		log := unittest.Logger()
		consumer := events.NewNoop()
		all := store.InitAll(metrics, pebbleimpl.ToDB(pdb))

		// bootstrap root snapshot
		state, err := pbadger.Bootstrap(
			metrics,
			pebbleimpl.ToDB(pdb),
			lockManager,
			all.Headers,
			all.Seals,
			all.Results,
			all.Blocks,
			all.QuorumCertificates,
			all.EpochSetups,
			all.EpochCommits,
			all.EpochProtocolStateEntries,
			all.ProtocolKVStore,
			all.VersionBeacons,
			rootSnapshot,
		)
		require.NoError(t, err)
		mockTimer := util.MockBlockTimer()

		// create follower state
		followerState, err := pbadger.NewFollowerState(
			log,
			tracer,
			consumer,
			state,
			all.Index,
			all.Payloads,
			mockTimer,
		)
		require.NoError(t, err)
		finalizer := moduleconsensus.NewFinalizer(pebbleimpl.ToDB(pdb).Reader(), all.Headers, followerState, tracer)
		rootHeader, err := rootSnapshot.Head()
		require.NoError(t, err)
		rootQC, err := rootSnapshot.QuorumCertificate()
		require.NoError(t, err)
		rootProtocolState, err := rootSnapshot.ProtocolState()
		require.NoError(t, err)
		rootProtocolStateID := rootProtocolState.ID()

		consensusConsumer := pubsub.NewFollowerDistributor()
		// use real consensus modules
		forks, err := consensus.NewForks(rootHeader, all.Headers, finalizer, consensusConsumer, rootHeader, rootQC)
		require.NoError(t, err)

		// assume all proposals are valid
		validator := mocks.NewValidator(t)
		validator.On("ValidateProposal", mock.Anything).Return(nil)

		// initialize the follower loop
		followerLoop, err := hotstuff.NewFollowerLoop(unittest.Logger(), metrics, forks)
		require.NoError(t, err)

		syncCore := module.NewBlockRequester(t)
		followerCore, err := NewComplianceCore(
			unittest.Logger(),
			metrics,
			metrics,
			consensusConsumer,
			followerState,
			followerLoop,
			validator,
			syncCore,
			tracer,
		)
		require.NoError(t, err)

		me := module.NewLocal(t)
		nodeID := unittest.IdentifierFixture()
		me.On("NodeID").Return(nodeID).Maybe()

		net := mocknetwork.NewNetwork(t)
		con := mocknetwork.NewConduit(t)
		net.On("Register", mock.Anything, mock.Anything).Return(con, nil)

		// use real engine
		engine, err := NewComplianceLayer(
			unittest.Logger(),
			net,
			me,
			metrics,
			all.Headers,
			rootHeader,
			followerCore,
			compliance.DefaultConfig(),
		)
		require.NoError(t, err)
		// don't forget to subscribe for finalization notifications
		consensusConsumer.AddOnBlockFinalizedConsumer(engine.OnFinalizedBlock)

		// start hotstuff logic and follower engine
		ctx, cancel, errs := irrecoverable.WithSignallerAndCancel(context.Background())
		followerLoop.Start(ctx)
		engine.Start(ctx)
		unittest.RequireCloseBefore(t, moduleutil.AllReady(engine, followerLoop), time.Second, "engine failed to start")

		// prepare chain of blocks, we will use a continuous chain assuming it was generated on happy path.
		workers := 5
		batchesPerWorker := 10
		blocksPerBatch := 100
		blocksPerWorker := blocksPerBatch * batchesPerWorker
		pendingBlocks := unittest.ProposalChainFixtureFrom(workers*blocksPerWorker, rootHeader)
		require.Greaterf(t, len(pendingBlocks), defaultPendingBlocksCacheCapacity, "this test assumes that we operate with more blocks than cache's upper limit")

		// ensure sequential block views - that way we can easily know which block will be finalized after the test
		for i, proposal := range pendingBlocks {
			proposal.Block.View = proposal.Block.Height
			proposal.Block.ParentView = proposal.Block.View - 1
			block, err := flow.NewBlock(
				flow.UntrustedBlock{
					HeaderBody: proposal.Block.HeaderBody,
					Payload:    unittest.PayloadFixture(unittest.WithProtocolStateID(rootProtocolStateID)),
				},
			)
			require.NoError(t, err)

			proposal.Block = *block

			if i > 0 {
				proposal.Block.ParentView = pendingBlocks[i-1].Block.View
				proposal.Block.ParentID = pendingBlocks[i-1].Block.ID()
			}
		}

		// Regarding the block that we expect to be finalized based on 2-chain finalization rule, we consider the last few blocks in `pendingBlocks`
		//  ... <-- X <-- Y <-- Z
		//            ╰─────────╯
		//          2-chain on top of X
		// Hence, we expect X to be finalized, which has the index `len(pendingBlocks)-3`
		// Note: the HotStuff Follower does not see block Z (as there is no QC for X proving its validity). Instead, it sees the certified block
		//  [◄(X) Y] ◄(Y)
		// where ◄(B) denotes a QC for block B
		targetBlockHeight := pendingBlocks[len(pendingBlocks)-3].Block.Height

		// emulate syncing logic, where we push same blocks over and over.
		originID := unittest.IdentifierFixture()
		submittingBlocks := atomic.NewBool(true)
		var wg sync.WaitGroup
		wg.Add(workers)
		for i := 0; i < workers; i++ {
			go func(blocks []*flow.Proposal) {
				defer wg.Done()
				for submittingBlocks.Load() {
					for batch := 0; batch < batchesPerWorker; batch++ {
						engine.OnSyncedBlocks(flow.Slashable[[]*flow.Proposal]{
							OriginID: originID,
							Message:  blocks[batch*blocksPerBatch : (batch+1)*blocksPerBatch],
						})
					}
				}
			}(pendingBlocks[i*blocksPerWorker : (i+1)*blocksPerWorker])
		}

		// wait for target block to become finalized, this might take a while.
		require.Eventually(t, func() bool {
			final, err := followerState.Final().Head()
			require.NoError(t, err)
			return final.Height == targetBlockHeight
		}, time.Minute, time.Second, "expect to process all blocks before timeout")

		// shutdown and cleanup test
		submittingBlocks.Store(false)
		unittest.RequireReturnsBefore(t, wg.Wait, time.Second, "expect workers to stop producing")
		cancel()
		unittest.RequireCloseBefore(t, moduleutil.AllDone(engine, followerLoop), time.Second, "engine failed to stop")
		select {
		case err := <-errs:
			require.NoError(t, err)
		default:
		}
	})
}
