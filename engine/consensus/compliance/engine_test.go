package compliance

import (
	"math/rand"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestComplianceEngine(t *testing.T) {
	suite.Run(t, new(ComplianceSuite))
}

type ComplianceSuite struct {
	ComplianceCoreSuite

	engine *Engine
}

func (cs *ComplianceSuite) SetupTest() {
	cs.ComplianceCoreSuite.SetupTest()
	log := zerolog.New(os.Stderr)
	e, err := NewEngine(log, cs.net, cs.me, cs.prov, cs.core)
	require.NoError(cs.T(), err)
	cs.engine = e

	ready := func() <-chan struct{} {
		channel := make(chan struct{})
		close(channel)
		return channel
	}()

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
	vote := messages.BlockVote{
		BlockID: blockID,
		View:    view,
		SigData: sig,
	}
	cs.con.AssertCalled(cs.T(), "Unicast", &vote, recipientID)
}

// TestBroadcastProposalWithDelay tests broadcasting proposals with different
// inputs
func (cs *ComplianceSuite) TestBroadcastProposalWithDelay() {

	// add execution node to participants to make sure we exclude them from broadcast
	cs.participants = append(cs.participants, unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution)))

	// generate a parent with height and chain ID set
	parent := unittest.BlockHeaderFixture()
	parent.ChainID = "test"
	parent.Height = 10
	cs.headerDB[parent.ID()] = &parent

	// create a block with the parent and store the payload with correct ID
	block := unittest.BlockWithParentFixture(&parent)
	block.Header.ProposerID = cs.myID
	cs.payloadDB[block.ID()] = block.Payload

	// keep a duplicate of the correct header to check against leader
	header := block.Header

	// unset chain and height to make sure they are correctly reconstructed
	block.Header.ChainID = ""
	block.Header.Height = 0

	cs.hotstuff.On("SubmitProposal", block.Header, parent.View).Return().Once()

	// submit to broadcast proposal
	err := cs.engine.BroadcastProposalWithDelay(block.Header, 0)
	require.NoError(cs.T(), err, "header broadcast should pass")

	// make sure chain ID and height were reconstructed and
	// we broadcast to correct nodes
	header.ChainID = "test"
	header.Height = 11
	msg := &messages.BlockProposal{
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
	cs.con.AssertCalled(cs.T(), "Publish", msg, cs.participants[1].NodeID, cs.participants[2].NodeID)

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

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		for i := 0; i < voteCount; i++ {
			vote := messages.BlockVote{
				BlockID: unittest.IdentifierFixture(),
				View:    rand.Uint64(),
				SigData: unittest.SignatureFixture(),
			}
			cs.hotstuff.On("SubmitVote", originID, vote.BlockID, vote.View, vote.SigData).Return()
			// execute the vote submission
			_ = cs.engine.Process(originID, &vote)
		}
		wg.Done()
	}()
	wg.Add(1)
	go func() {
		// create a proposal that directly descends from the latest finalized header
		originID := cs.participants[1].NodeID
		block := unittest.BlockWithParentFixture(cs.head)
		proposal := unittest.ProposalFromBlock(&block)

		// store the data for retrieval
		cs.headerDB[block.Header.ParentID] = cs.head
		cs.hotstuff.On("SubmitProposal", block.Header, cs.head.View).Return()
		_ = cs.engine.Process(originID, proposal)
		wg.Done()
	}()

	wg.Wait()

	time.Sleep(time.Second)

	// check the submit vote was called with correct parameters
	cs.hotstuff.AssertExpectations(cs.T())
}
