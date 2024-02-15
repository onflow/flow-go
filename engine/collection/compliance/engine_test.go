package compliance

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module/irrecoverable"
	module "github.com/onflow/flow-go/module/mock"
	netint "github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/mocknetwork"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	storerr "github.com/onflow/flow-go/storage"
	storage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestComplianceEngine(t *testing.T) {
	suite.Run(t, new(EngineSuite))
}

type EngineSuite struct {
	CommonSuite

	clusterID  flow.ChainID
	myID       flow.Identifier
	cluster    flow.IdentityList
	me         *module.Local
	net        *mocknetwork.Network
	payloads   *storage.ClusterPayloads
	protoState *protocol.State
	con        *mocknetwork.Conduit

	payloadDB map[flow.Identifier]*cluster.Payload

	ctx    irrecoverable.SignalerContext
	cancel context.CancelFunc
	errs   <-chan error

	engine *Engine
}

func (cs *EngineSuite) SetupTest() {
	cs.CommonSuite.SetupTest()

	// initialize the parameters
	cs.cluster = unittest.IdentityListFixture(3,
		unittest.WithRole(flow.RoleCollection),
		unittest.WithInitialWeight(1000),
	)
	cs.myID = cs.cluster[0].NodeID

	protoEpoch := &protocol.Epoch{}
	clusters := flow.ClusterList{cs.cluster.ToSkeleton()}
	protoEpoch.On("Clustering").Return(clusters, nil)

	protoQuery := &protocol.EpochQuery{}
	protoQuery.On("Current").Return(protoEpoch)

	protoSnapshot := &protocol.Snapshot{}
	protoSnapshot.On("Epochs").Return(protoQuery)
	protoSnapshot.On("Identities", mock.Anything).Return(
		func(selector flow.IdentityFilter[flow.Identity]) flow.IdentityList {
			return cs.cluster.Filter(selector)
		},
		nil,
	)

	cs.protoState = &protocol.State{}
	cs.protoState.On("Final").Return(protoSnapshot)

	cs.clusterID = "cluster-id"
	clusterParams := &protocol.Params{}
	clusterParams.On("ChainID").Return(cs.clusterID, nil)

	cs.state.On("Params").Return(clusterParams)

	// set up local module mock
	cs.me = &module.Local{}
	cs.me.On("NodeID").Return(
		func() flow.Identifier {
			return cs.myID
		},
	)

	cs.payloadDB = make(map[flow.Identifier]*cluster.Payload)

	// set up payload storage mock
	cs.payloads = &storage.ClusterPayloads{}
	cs.payloads.On("Store", mock.Anything, mock.Anything).Return(
		func(blockID flow.Identifier, payload *cluster.Payload) error {
			cs.payloadDB[blockID] = payload
			return nil
		},
	)
	cs.payloads.On("ByBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) *cluster.Payload {
			return cs.payloadDB[blockID]
		},
		func(blockID flow.Identifier) error {
			_, exists := cs.payloadDB[blockID]
			if !exists {
				return storerr.ErrNotFound
			}
			return nil
		},
	)

	// set up network conduit mock
	cs.con = &mocknetwork.Conduit{}
	cs.con.On("Publish", mock.Anything, mock.Anything).Return(nil)
	cs.con.On("Publish", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	cs.con.On("Publish", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	cs.con.On("Unicast", mock.Anything, mock.Anything).Return(nil)

	// set up network module mock
	cs.net = &mocknetwork.Network{}
	cs.net.On("Register", mock.Anything, mock.Anything).Return(
		func(channel channels.Channel, engine netint.MessageProcessor) netint.Conduit {
			return cs.con
		},
		nil,
	)

	e, err := NewEngine(unittest.Logger(), cs.me, cs.protoState, cs.payloads, cs.core)
	require.NoError(cs.T(), err)
	cs.engine = e

	cs.ctx, cs.cancel, cs.errs = irrecoverable.WithSignallerAndCancel(context.Background())
	cs.engine.Start(cs.ctx)
	go unittest.FailOnIrrecoverableError(cs.T(), cs.ctx.Done(), cs.errs)

	unittest.AssertClosesBefore(cs.T(), cs.engine.Ready(), time.Second)
}

// TearDownTest stops the engine and checks there are no errors thrown to the SignallerContext.
func (cs *EngineSuite) TearDownTest() {
	cs.cancel()
	unittest.RequireCloseBefore(cs.T(), cs.engine.Done(), time.Second, "engine failed to stop")
	select {
	case err := <-cs.errs:
		assert.NoError(cs.T(), err)
	default:
	}
}

// TestSubmittingMultipleVotes tests that we can send multiple votes and they
// are queued and processed in expected way
func (cs *EngineSuite) TestSubmittingMultipleEntries() {
	blockCount := 15
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		for i := 0; i < blockCount; i++ {
			block := unittest.ClusterBlockWithParent(cs.head)
			proposal := messages.NewClusterBlockProposal(&block)
			hotstuffProposal := model.ProposalFromFlow(block.Header)
			cs.hotstuff.On("SubmitProposal", hotstuffProposal).Return().Once()
			cs.voteAggregator.On("AddBlock", hotstuffProposal).Once()
			cs.validator.On("ValidateProposal", hotstuffProposal).Return(nil).Once()
			// execute the block submission
			cs.engine.OnClusterBlockProposal(flow.Slashable[*messages.ClusterBlockProposal]{
				OriginID: unittest.IdentifierFixture(),
				Message:  proposal,
			})
		}
		wg.Done()
	}()
	wg.Add(1)
	go func() {
		// create a proposal that directly descends from the latest finalized header
		block := unittest.ClusterBlockWithParent(cs.head)
		proposal := messages.NewClusterBlockProposal(&block)

		hotstuffProposal := model.ProposalFromFlow(block.Header)
		cs.hotstuff.On("SubmitProposal", hotstuffProposal).Once()
		cs.voteAggregator.On("AddBlock", hotstuffProposal).Once()
		cs.validator.On("ValidateProposal", hotstuffProposal).Return(nil).Once()
		cs.engine.OnClusterBlockProposal(flow.Slashable[*messages.ClusterBlockProposal]{
			OriginID: unittest.IdentifierFixture(),
			Message:  proposal,
		})
		wg.Done()
	}()

	wg.Wait()

	// wait for the votes queue to drain
	assert.Eventually(cs.T(), func() bool {
		return cs.engine.pendingBlocks.Len() == 0
	}, time.Second, time.Millisecond*10)
}

// TestOnFinalizedBlock tests if finalized block gets processed when send through `Engine`.
// Tests the whole processing pipeline.
func (cs *EngineSuite) TestOnFinalizedBlock() {
	finalizedBlock := unittest.ClusterBlockFixture()
	cs.head = &finalizedBlock
	cs.headerDB[finalizedBlock.ID()] = &finalizedBlock

	*cs.pending = module.PendingClusterBlockBuffer{}
	// wait for both expected calls before ending the test
	wg := new(sync.WaitGroup)
	wg.Add(2)
	cs.pending.On("PruneByView", finalizedBlock.Header.View).
		Run(func(_ mock.Arguments) { wg.Done() }).
		Return(nil).Once()
	cs.pending.On("Size").
		Run(func(_ mock.Arguments) { wg.Done() }).
		Return(uint(0)).Once()

	err := cs.engine.processOnFinalizedBlock(model.BlockFromFlow(finalizedBlock.Header))
	require.NoError(cs.T(), err)
	unittest.AssertReturnsBefore(cs.T(), wg.Wait, time.Second, "an expected call to block buffer wasn't made")
}
