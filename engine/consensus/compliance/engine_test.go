package compliance

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"math/rand"
	"testing"
	"time"
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
	e, err := NewEngine(cs.net, cs.me, cs.prov, cs.core)
	require.NoError(cs.T(), err)
	cs.engine = e
}

func (cs *ComplianceSuite) TestSendVote() {
	// create parameters to send a vote
	blockID := unittest.IdentifierFixture()
	view := rand.Uint64()
	sig := unittest.SignatureFixture()
	recipientID := unittest.IdentifierFixture()

	// submit the vote
	err := cs.engine.SendVote(blockID, view, sig, recipientID)
	require.NoError(cs.T(), err, "should pass send vote")

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
