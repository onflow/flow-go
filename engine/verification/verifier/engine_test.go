package verifier

import (
	"errors"
	"fmt"
	"math/rand"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/execution"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/identity"
	"github.com/dapperlabs/flow-go/model/verification"
	"github.com/dapperlabs/flow-go/module/mock"
	network "github.com/dapperlabs/flow-go/network/mock"
	protocol "github.com/dapperlabs/flow-go/protocol/mock"
	testifymock "github.com/stretchr/testify/mock"
)

// Test suit for functionality testing of Verifier Engine
type VerifierEngineTestSuit struct {
	suite.Suite
	e     *Engine       // the mock engine, used for testing different functionalities
	net   *mock.Network // used as an instance of networking layer for the mock engine
	state *protocol.State
	me    *mock.Local
	con   *network.Conduit
	ss    *protocol.Snapshot
}

// SetupTests initiates the test setups prior to each test
func (suite *VerifierEngineTestSuit) SetupTest() {
	suite.state = &protocol.State{} // used as a mock identity table for the mock engine
	suite.con = &network.Conduit{}  // used as a mock conduit for the mock engine
	suite.net = &mock.Network{}     // used as a mock network for the mock engine
	suite.me = &mock.Local{}
	suite.ss = &protocol.Snapshot{}
	log := zerolog.Logger{} // used as to log relevant

	// the mock verifier engine
	suite.e = &Engine{
		log:   log,
		me:    suite.me,
		state: suite.state,
	}

	// mocking the network registration of the engine
	suite.net.On("Register", uint8(engine.VerificationVerifier), suite.e).
		Return(suite.con, nil).
		Once()
}

func TestVerifierEngineTestSuite(t *testing.T) {
	suite.Run(t, new(VerifierEngineTestSuit))
}

// TestNewEngine_NetworkRegistration verifies the establishment of the network registration upon
// creation of an instance of verifier.Engine using the New method
func (suite *VerifierEngineTestSuit) TestNewEngine_NetworkRegistration() {
	// creating a new engine
	_, err := New(suite.e.log, suite.net, suite.e.state, suite.e.me)
	require.Nil(suite.T(), err, "could not create an engine")
	suite.net.AssertExpectations(suite.T())
}

// Test_Submit_HappyPath covers the happy path of submitting a valid execution receipt to
// a single verifier engine till a result approval is emitted to all the consensus nodes
func (suite *VerifierEngineTestSuit) Test_Submit_HappyPath() {
	// creating a new engine
	vrfy, err := New(suite.e.log, suite.net, suite.e.state, suite.e.me)
	require.Nil(suite.T(), err, "could not create an engine")

	//mocking the identity of the verification node under test
	vn_me := flow.Identity{
		NodeID:  flow.Identifier{0x01, 0x01, 0x01, 0x01},
		Address: "mock-vn-address",
		Role:    flow.RoleVerification,
	}
	//mocking state for me.NodeID for twice
	suite.me.On("NodeID").Return(vn_me.NodeID).Twice()

	// mocking for Final().Identities(Identity(verifierNode))
	suite.state.On("Final").Return(suite.ss).Once()
	suite.ss.On("Identity", vn_me.NodeID).Return(vn_me, nil).Once()

	// a set of mock staked nodes
	ids := generateMockIdentities(100)
	cons_id := ids.Filter(identity.HasRole(flow.RoleConsensus))

	// mocking for Final().Identities(identity.HasRole(flow.RoleConsensus))
	suite.state.On("Final").Return(suite.ss).Once()
	suite.ss.On("Identities", testifymock.Anything).Return(cons_id, nil)

	// extracting mock consensus nodes IDs
	params := []interface{}{&verification.ResultApproval{}}
	for _, targetID := range cons_id.NodeIDs() {
		params = append(params, targetID)
	}

	// the happy path ends by the verifier engine emitting a
	// result approval to ONLY all the consensus nodes
	suite.con.On("Submit", params...).
		Return(nil).
		Once()

	// emitting an execution receipt form the execution node
	vrfy.Submit(&execution.ExecutionReceipt{})

	suite.state.AssertExpectations(suite.T())
	suite.con.AssertExpectations(suite.T())
	suite.ss.AssertExpectations(suite.T())
	suite.me.AssertExpectations(suite.T())
}

// TestProcess_UnhappyInput covers unhappy inputs for Process method
func (suite *VerifierEngineTestSuit) TestProcess_UnhappyInput() {
	// mocking state for Final().Identity(flow.Identifier{})
	suite.state.On("Final").Return(suite.ss).Once()
	suite.ss.On("Identity", flow.Identifier{}).Return(flow.Identity{}, errors.New("non-nil")).Once()

	// creating a new engine
	vrfy, err := New(suite.e.log, suite.net, suite.e.state, suite.me)
	require.Nil(suite.T(), err, "could not create an engine")

	// nil event
	err = vrfy.Process(flow.Identifier{}, nil)
	assert.NotNil(suite.T(), err, "failed recognizing nil event")

	// non-execution receipt event
	err = vrfy.Process(flow.Identifier{}, new(struct{}))
	assert.NotNil(suite.T(), err, "failed recognizing non-execution receipt events")

	// non-recoverable id
	err = vrfy.Process(flow.Identifier{}, &execution.ExecutionReceipt{})
	assert.NotNilf(suite.T(), err, "broken happy path: %s", err)

	// asserting a single calls in unhappy path
	suite.net.AssertExpectations(suite.T())
	suite.state.AssertExpectations(suite.T())
	suite.ss.AssertExpectations(suite.T())
}

// TestProcess_UnstakeEmit tests the Process method of Verifier engine against
// an unauthorized node emitting an execution receipt. The process method should
// catch this injected fault by returning an error
func (suite *VerifierEngineTestSuit) TestProcess_UnstakeEmit() {
	// creating a new engine
	vrfy, err := New(suite.e.log, suite.net, suite.e.state, suite.me)
	require.Nil(suite.T(), err, "could not create an engine")

	unstaked_id := flow.Identity{
		NodeID:  flow.Identifier{0x02, 0x02, 0x02, 0x02},
		Address: "unstaked_address",
		Role:    flow.RoleExecution,
		Stake:   0,
	}

	// mocking state for Final().Identity(unstaked_id.NodeID)
	suite.state.On("Final").Return(suite.ss).Once()
	suite.ss.On("Identity", unstaked_id.NodeID).
		Return(flow.Identity{}, errors.New("non-nil")).Once()

	// execution receipts should directly come from Execution Nodes,
	// hence for all test cases a non-nil error should returned
	err = vrfy.Process(unstaked_id.NodeID, &execution.ExecutionReceipt{})
	assert.NotNil(suite.T(), err, "failed rejecting an unstaked id")
	suite.state.AssertExpectations(suite.T())
	suite.ss.AssertExpectations(suite.T())
}

// TestProcess_UnauthorizedEmits follows the unhappy path where staked nodes
// rather than execution nodes send an execution receipt event
func (suite *VerifierEngineTestSuit) TestProcess_UnauthorizedEmits() {
	// defining mock nodes identities
	// test table covers all roles except the execution nodes
	// that are the only legitimate party to originate an execution receipt
	tt := []struct {
		role flow.Role //the test input
		err  error     //expected test result
	}{
		{ // consensus node
			role: flow.RoleConsensus,
			err:  errors.New("non-nil"),
		},
		{ // observer node
			role: flow.RoleObservation,
			err:  errors.New("non-nil"),
		},
		{ // collection node
			role: flow.RoleCollection,
			err:  errors.New("non-nil"),
		},
		{ // verification node
			role: flow.RoleVerification,
			err:  errors.New("non-nil"),
		},
	}

	//mocking the identity of the verification node under test
	vn_me := flow.Identity{
		NodeID:  flow.Identifier{0x01, 0x01, 0x01, 0x01},
		Address: "mock-vn-address",
		Role:    flow.RoleVerification,
	}

	// creating a new engine
	vrfy, err := New(suite.e.log, suite.net, suite.e.state, suite.e.me)
	require.Nil(suite.T(), err, "could not create an engine")

	for _, tc := range tt {
		id := flow.Identity{
			NodeID:  flow.Identifier{0x02, 0x02, 0x02, 0x02},
			Address: "mock-address",
			Role:    tc.role,
		}
		// mocking state fo Final().Identity(originID)
		suite.state.On("Final").Return(suite.ss).Once()
		suite.ss.On("Identity", id.NodeID).Return(id, nil).Once()

		//mocking state for e.me.NodeID(vn_me.NodeID) for twice
		suite.me.On("NodeID").Return(vn_me.NodeID).Once()

		// execution receipts should directly come from Execution Nodes,
		// hence for all test cases a non-nil error should returned
		err = vrfy.Process(id.NodeID, &execution.ExecutionReceipt{})
		assert.NotNil(suite.T(), err, "failed rejecting an faulty origin id")
		suite.state.AssertExpectations(suite.T())
		suite.ss.AssertExpectations(suite.T())
		suite.me.AssertExpectations(suite.T())
	}
}

// TestOnExecutionReceipt_HappyPath covers the happy path of the verifier engine on receiving an valid execution receipt
// The expected behavior is to verify the receipt and emit a result approval to all consensus nodes
func (suite *VerifierEngineTestSuit) TestOnExecutionReceipt_HappyPath() {
	// creating a new engine
	vrfy, err := New(suite.e.log, suite.net, suite.e.state, suite.e.me)
	require.Nil(suite.T(), err, "could not create an engine")

	// a mock staked execution node for generating a mock execution receipt
	exe_id := flow.Identity{
		NodeID:  flow.Identifier{0x02, 0x02, 0x02, 0x02},
		Address: "mock-en-address",
		Role:    flow.RoleExecution,
	}

	// mocking state fo Final().Identity(originID)
	suite.state.On("Final").Return(suite.ss).Once()
	suite.ss.On("Identity", exe_id.NodeID).Return(exe_id, nil).Once()

	// a set of mock staked nodes
	ids := generateMockIdentities(100)
	// extracting mock consensus nodes IDs
	cons_ids := ids.Filter(identity.HasRole(flow.RoleConsensus))

	// mocking for Final().Identities(identity.HasRole(flow.RoleConsensus))
	suite.state.On("Final").Return(suite.ss).Once()
	suite.ss.On("Identities", testifymock.Anything).Return(cons_ids, nil)

	params := []interface{}{&verification.ResultApproval{}}
	for _, targetID := range cons_ids {
		params = append(params, targetID.NodeID)
	}

	// the happy path ends by the verifier engine emitting a
	// result approval to ONLY all the consensus nodes
	suite.con.On("Submit", params...).
		Return(nil).
		Once()

	// emitting an execution receipt form the execution node
	err = vrfy.Process(exe_id.NodeID, &execution.ExecutionReceipt{})
	assert.Nil(suite.T(), err, "failed processing execution receipt")

	suite.con.AssertExpectations(suite.T())
	suite.ss.AssertExpectations(suite.T())
	suite.state.AssertExpectations(suite.T())
}

// generateMockIdentities generates and returns set of random nodes with different roles
// the distribution of the roles is unifors m but not guaranteed
// size: total number of nodes
func generateMockIdentities(size int) flow.IdentityList {
	var identities flow.IdentityList
	for i := 0; i < size; i++ {
		// creating mock identities as a random byte array
		var nodeID flow.Identifier
		_, _ = rand.Read(nodeID[:])
		address := fmt.Sprintf("address%d", i)
		var role flow.Role
		switch rand.Intn(5) {
		case 0:
			role = flow.RoleCollection
		case 1:
			role = flow.RoleConsensus
		case 2:
			role = flow.RoleExecution
		case 3:
			role = flow.RoleVerification
		case 4:
			role = flow.RoleObservation
		}
		id := flow.Identity{
			NodeID:  nodeID,
			Address: address,
			Role:    role,
		}
		identities = append(identities, id)
	}
	return identities
}
