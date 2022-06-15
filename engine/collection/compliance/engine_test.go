package compliance

import (
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	module "github.com/onflow/flow-go/module/mock"
	netint "github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/mocknetwork"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	storerr "github.com/onflow/flow-go/storage"
	storage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestComplianceEngine(t *testing.T) {
	suite.Run(t, new(ComplianceSuite))
}

type ComplianceSuite struct {
	ComplianceCoreSuite

	clusterID  flow.ChainID
	myID       flow.Identifier
	cluster    flow.IdentityList
	me         *module.Local
	net        *mocknetwork.Network
	payloads   *storage.ClusterPayloads
	protoState *protocol.MutableState
	con        *mocknetwork.Conduit

	payloadDB map[flow.Identifier]*cluster.Payload

	engine *Engine
}

func (cs *ComplianceSuite) SetupTest() {
	cs.ComplianceCoreSuite.SetupTest()

	// initialize the paramaters
	cs.cluster = unittest.IdentityListFixture(3,
		unittest.WithRole(flow.RoleCollection),
		unittest.WithWeight(1000),
	)
	cs.myID = cs.cluster[0].NodeID

	protoEpoch := &protocol.Epoch{}
	clusters := flow.ClusterList{cs.cluster}
	protoEpoch.On("Clustering").Return(clusters, nil)

	protoQuery := &protocol.EpochQuery{}
	protoQuery.On("Current").Return(protoEpoch)

	protoSnapshot := &protocol.Snapshot{}
	protoSnapshot.On("Epochs").Return(protoQuery)
	protoSnapshot.On("Identities", mock.Anything).Return(
		func(selector flow.IdentityFilter) flow.IdentityList {
			return cs.cluster.Filter(selector)
		},
		nil,
	)

	cs.protoState = &protocol.MutableState{}
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
		func(channel netint.Channel, engine netint.MessageProcessor) netint.Conduit {
			return cs.con
		},
		nil,
	)

	e, err := NewEngine(unittest.Logger(), cs.net, cs.me, cs.protoState, cs.payloads, cs.core)
	require.NoError(cs.T(), err)
	cs.engine = e

	ready := func() <-chan struct{} {
		channel := make(chan struct{})
		close(channel)
		return channel
	}()

	cs.hotstuff.On("Start", mock.Anything)
	cs.hotstuff.On("Ready", mock.Anything).Return(ready)
	<-cs.engine.Ready()
}

// TestSendVote tests that single vote can be send and properly processed
func (cs *ComplianceSuite) TestSendVote() {
	// create parameters to send a vote
	blockID := unittest.IdentifierFixture()
	view := rand.Uint64()
	sig := unittest.SignatureFixture()
	recipientID := unittest.IdentifierFixture()

	// submit the vote
	err := cs.engine.SendVote(blockID, view, sig, recipientID)
	require.NoError(cs.T(), err, "should pass send vote")

	done := func() <-chan struct{} {
		channel := make(chan struct{})
		close(channel)
		return channel
	}()

	cs.hotstuff.On("Done", mock.Anything).Return(done)

	// The vote is transmitted asynchronously. We allow 10ms for the vote to be received:
	<-time.After(10 * time.Millisecond)
	<-cs.engine.Done()

	// check it was called with right params
	vote := messages.ClusterBlockVote{
		BlockID: blockID,
		View:    view,
		SigData: sig,
	}
	cs.con.AssertCalled(cs.T(), "Unicast", &vote, recipientID)
}

// TestBroadcastProposalWithDelay tests broadcasting proposals with different
// inputs
func (cs *ComplianceSuite) TestBroadcastProposalWithDelay() {

	// generate a parent with height and chain ID set
	parent := unittest.ClusterBlockFixture()
	parent.Header.ChainID = "test"
	parent.Header.Height = 10
	cs.headerDB[parent.ID()] = &parent

	// create a block with the parent and store the payload with correct ID
	block := unittest.ClusterBlockWithParent(&parent)
	block.Header.ProposerID = cs.myID
	cs.payloadDB[block.ID()] = block.Payload

	// keep a duplicate of the correct header to check against leader
	header := block.Header

	// unset chain and height to make sure they are correctly reconstructed
	block.Header.ChainID = ""
	block.Header.Height = 0

	cs.hotstuff.On("SubmitProposal", block.Header, parent.Header.View).Return().Once()

	// submit to broadcast proposal
	err := cs.engine.BroadcastProposalWithDelay(block.Header, 0)
	require.NoError(cs.T(), err, "header broadcast should pass")

	// make sure chain ID and height were reconstructed and
	// we broadcast to correct nodes
	header.ChainID = "test"
	header.Height = 11
	msg := &messages.ClusterBlockProposal{
		Header:  header,
		Payload: block.Payload,
	}

	done := func() <-chan struct{} {
		channel := make(chan struct{})
		close(channel)
		return channel
	}()

	cs.hotstuff.On("Done", mock.Anything).Return(done)

	<-time.After(10 * time.Millisecond)
	<-cs.engine.Done()
	cs.con.AssertCalled(cs.T(), "Publish", msg, cs.cluster[1].NodeID, cs.cluster[2].NodeID)

	// should fail with wrong proposer
	header.ProposerID = unittest.IdentifierFixture()
	err = cs.engine.BroadcastProposalWithDelay(header, 0)
	require.Error(cs.T(), err, "should fail with wrong proposer")
	header.ProposerID = cs.myID

	// should fail with changed (missing) parent
	header.ParentID[0]++
	err = cs.engine.BroadcastProposalWithDelay(header, 0)
	require.Error(cs.T(), err, "should fail with missing parent")
	header.ParentID[0]--

	// should fail with wrong block ID (payload unavailable)
	header.View++
	err = cs.engine.BroadcastProposalWithDelay(header, 0)
	require.Error(cs.T(), err, "should fail with missing payload")
	header.View--
}

// TestSubmittingMultipleVotes tests that we can send multiple votes and they
// are queued and processed in expected way
func (cs *ComplianceSuite) TestSubmittingMultipleEntries() {
	// create a vote
	originID := unittest.IdentifierFixture()
	voteCount := 15

	channel := engine.ChannelConsensusCluster(cs.clusterID)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		for i := 0; i < voteCount; i++ {
			vote := messages.ClusterBlockVote{
				BlockID: unittest.IdentifierFixture(),
				View:    rand.Uint64(),
				SigData: unittest.SignatureFixture(),
			}
			cs.voteAggregator.On("AddVote", &model.Vote{
				View:     vote.View,
				BlockID:  vote.BlockID,
				SignerID: originID,
				SigData:  vote.SigData,
			}).Return().Once()
			// execute the vote submission
			_ = cs.engine.Process(channel, originID, &vote)
		}
		wg.Done()
	}()
	wg.Add(1)
	go func() {
		// create a proposal that directly descends from the latest finalized header
		originID := cs.cluster[1].NodeID
		block := unittest.ClusterBlockWithParent(cs.head)
		proposal := &messages.ClusterBlockProposal{
			Header:  block.Header,
			Payload: block.Payload,
		}

		// store the data for retrieval
		cs.headerDB[block.Header.ParentID] = cs.head
		cs.hotstuff.On("SubmitProposal", block.Header, cs.head.Header.View).Return()
		_ = cs.engine.Process(channel, originID, proposal)
		wg.Done()
	}()

	wg.Wait()

	time.Sleep(time.Second)

	// check that submit vote was called with correct parameters
	cs.hotstuff.AssertExpectations(cs.T())
	cs.voteAggregator.AssertExpectations(cs.T())
}

// TestProcessUnsupportedMessageType tests that Process and ProcessLocal correctly handle a case where invalid message type
// was submitted from network layer.
func (cs *ComplianceSuite) TestProcessUnsupportedMessageType() {
	invalidEvent := uint64(42)
	err := cs.engine.Process("ch", unittest.IdentifierFixture(), invalidEvent)
	// shouldn't result in error since byzantine inputs are expected
	require.NoError(cs.T(), err)
	// in case of local processing error cannot be consumed since all inputs are trusted
	err = cs.engine.ProcessLocal(invalidEvent)
	require.Error(cs.T(), err)
	require.True(cs.T(), engine.IsIncompatibleInputTypeError(err))
}

// TestOnFinalizedBlock tests if finalized block gets processed when send through `Engine`.
// Tests the whole processing pipeline.
func (cs *ComplianceSuite) TestOnFinalizedBlock() {
	finalizedBlock := unittest.ClusterBlockFixture()
	cs.head = &finalizedBlock

	*cs.pending = module.PendingClusterBlockBuffer{}
	cs.pending.On("PruneByView", finalizedBlock.Header.View).Return(nil).Once()
	cs.pending.On("Size").Return(uint(0)).Once()
	cs.engine.OnFinalizedBlock(model.BlockFromFlow(finalizedBlock.Header, finalizedBlock.Header.View-1))

	require.Eventually(cs.T(),
		func() bool {
			return cs.pending.AssertCalled(cs.T(), "PruneByView", finalizedBlock.Header.View)
		}, time.Second, time.Millisecond*20)
}
