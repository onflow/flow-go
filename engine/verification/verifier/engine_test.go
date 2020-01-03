package verifier

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/verification"
	"github.com/dapperlabs/flow-go/engine/verification/storage"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/identity"
	"github.com/dapperlabs/flow-go/module/mock"
	network "github.com/dapperlabs/flow-go/network/mock"
	protocol "github.com/dapperlabs/flow-go/protocol/mock"
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
	wg    *sync.WaitGroup
}

// SetupTests initiates the test setups prior to each test
func (v *VerifierEngineTestSuit) SetupTest() {
	v.state = &protocol.State{} // used as a mock identity table for the mock engine
	v.con = &network.Conduit{}  // used as a mock conduit for the mock engine
	v.net = &mock.Network{}     // used as a mock network for the mock engine
	v.me = &mock.Local{}
	v.ss = &protocol.Snapshot{}
	log := zerolog.Logger{} // used as to log relevant
	wg := sync.WaitGroup{}

	// the mock verifier engine
	v.e = &Engine{
		unit:  engine.NewUnit(),
		log:   log,
		state: v.state,
		me:    v.me,
		store: *storage.New(),
		wg:    wg,
		mu:    sync.Mutex{},
	}

	// mocking the network registration of the engine
	v.net.On("Register", uint8(engine.VerificationVerifier), testifymock.Anything).
		Return(v.con, nil).
		Once()
}

func TestVerifierEngineTestSuite(t *testing.T) {
	suite.Run(t, new(VerifierEngineTestSuit))
}

// TestNewEngineNetworkRegistration verifies the establishment of the network registration upon
// creation of an instance of verifier.Engine using the New method
func (v *VerifierEngineTestSuit) TestNewEngineNetworkRegistration() {
	// creating a new engine
	_, err := New(v.e.log, v.net, v.e.state, v.e.me)
	require.Nil(v.T(), err, "could not create an engine")
	v.net.AssertExpectations(v.T())

	// also independently testing the encoding of result approvals
	// this is just to ensure that we have an encodable result approval
	resApprove := flow.ResultApproval{}
	resApprove.Fingerprint()
}

// TestSubmitHappyPath covers the happy path of submitting a valid execution receipt to
// a single verifier engine till a result approval is emitted to all the consensus nodes
func (v *VerifierEngineTestSuit) TestSubmitHappyPath() {
	// creating a new engine
	vrfy, err := New(v.e.log, v.net, v.e.state, v.e.me)
	require.Nil(v.T(), err, "could not create an engine")

	//mocking the identity of the verification node under test
	vnMe := flow.Identity{
		NodeID:  flow.Identifier{0x01, 0x01, 0x01, 0x01},
		Address: "mock-vn-address",
		Role:    flow.RoleVerification,
	}
	//mocking state for me.NodeID for twice
	v.me.On("NodeID").Return(vnMe.NodeID).Twice()

	// mocking for Final().Identities(Identity(verifierNode))
	v.state.On("Final").Return(v.ss).Once()
	v.ss.On("Identity", vnMe.NodeID).Return(vnMe, nil).Once()

	// a set of mock staked nodes
	ids := generateMockIdentities(100)
	consID := ids.Filter(identity.HasRole(flow.RoleConsensus))

	// mocking for Final().Identities(identity.HasRole(flow.RoleConsensus))
	v.state.On("Final").Return(v.ss).Once()
	v.ss.On("Identities", testifymock.Anything).Return(consID, nil)

	// generating a random ER and its associated result approval
	er := verification.RandomERGen()
	restApprov := verification.RnadRAGen(er)
	// extracting mock consensus nodes IDs
	params := []interface{}{restApprov}
	for _, targetID := range consID.NodeIDs() {
		params = append(params, targetID)
	}

	// the happy path ends by the verifier engine emitting a
	// result approval to ONLY all the consensus nodes
	v.con.On("Submit", params...).
		Return(nil).
		Once()

	// store of the engine should be empty prior to the submit
	assert.Equal(v.T(), vrfy.store.ResultsNum(), 0)

	// emitting an execution receipt form the execution node
	_ = vrfy.ProcessLocal(er)

	// store of the engine should be of size one prior to the submit
	assert.Equal(v.T(), vrfy.store.ResultsNum(), 1)

	vrfy.wg.Wait()
	v.state.AssertExpectations(v.T())
	v.con.AssertExpectations(v.T())
	v.ss.AssertExpectations(v.T())
	v.me.AssertExpectations(v.T())
}

// TestProcessUnhappyInput covers unhappy inputs for Process method
func (v *VerifierEngineTestSuit) TestProcessUnhappyInput() {
	// mocking state for Final().Identity(flow.Identifier{})
	v.state.On("Final").Return(v.ss).Once()
	v.ss.On("Identity", flow.Identifier{}).Return(flow.Identity{}, errors.New("non-nil")).Once()

	// creating a new engine
	vrfy, err := New(v.e.log, v.net, v.e.state, v.me)
	require.Nil(v.T(), err, "could not create an engine")

	// nil event
	err = vrfy.Process(flow.Identifier{}, nil)
	assert.NotNil(v.T(), err, "failed recognizing nil event")

	// non-execution receipt event
	err = vrfy.Process(flow.Identifier{}, new(struct{}))
	assert.NotNil(v.T(), err, "failed recognizing non-execution receipt events")

	// non-recoverable id
	err = vrfy.Process(flow.Identifier{}, &flow.ExecutionReceipt{})
	assert.NotNilf(v.T(), err, "broken happy path: %s", err)

	// asserting a single calls in unhappy path
	vrfy.wg.Wait()
	v.net.AssertExpectations(v.T())
	v.state.AssertExpectations(v.T())
	v.ss.AssertExpectations(v.T())
}

// TestProcessUnstakeEmit tests the Process method of Verifier engine against
// an unauthorized node emitting an execution receipt. The process method should
// catch this injected fault by returning an error
func (v *VerifierEngineTestSuit) TestProcessUnstakeEmit() {
	// creating a new engine
	vrfy, err := New(v.e.log, v.net, v.e.state, v.me)
	require.Nil(v.T(), err, "could not create an engine")

	unstakedID := flow.Identity{
		NodeID:  flow.Identifier{0x02, 0x02, 0x02, 0x02},
		Address: "unstaked_address",
		Role:    flow.RoleExecution,
		Stake:   0,
	}

	// mocking state for Final().Identity(unstakedID.NodeID)
	v.state.On("Final").Return(v.ss).Once()
	v.ss.On("Identity", unstakedID.NodeID).
		Return(flow.Identity{}, errors.New("non-nil")).Once()

	// execution receipts should directly come from Execution Nodes,
	// hence for all test cases a non-nil error should returned
	err = vrfy.Process(unstakedID.NodeID, &flow.ExecutionReceipt{})
	assert.NotNil(v.T(), err, "failed rejecting an unstaked id")

	vrfy.wg.Wait()
	v.state.AssertExpectations(v.T())
	v.ss.AssertExpectations(v.T())
}

// TestProcessUnauthorizedEmits follows the unhappy path where staked nodes
// rather than execution nodes send an execution receipt event
func (v *VerifierEngineTestSuit) TestProcessUnauthorizedEmits() {
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
	vnMe := flow.Identity{
		NodeID:  flow.Identifier{0x01, 0x01, 0x01, 0x01},
		Address: "mock-vn-address",
		Role:    flow.RoleVerification,
	}

	// creating a new engine
	vrfy, err := New(v.e.log, v.net, v.e.state, v.e.me)
	require.Nil(v.T(), err, "could not create an engine")

	for _, tc := range tt {
		id := flow.Identity{
			NodeID:  flow.Identifier{0x02, 0x02, 0x02, 0x02},
			Address: "mock-address",
			Role:    tc.role,
		}
		// mocking state fo Final().Identity(originID)
		v.state.On("Final").Return(v.ss).Once()
		v.ss.On("Identity", id.NodeID).Return(id, nil).Once()

		//mocking state for e.me.NodeID(vnMe.NodeID) for twice
		v.me.On("NodeID").Return(vnMe.NodeID).Once()

		// execution receipts should directly come from Execution Nodes,
		// hence for all test cases a non-nil error should returned
		err = vrfy.Process(id.NodeID, &flow.ExecutionReceipt{})
		assert.NotNil(v.T(), err, "failed rejecting an faulty origin id")

		vrfy.wg.Wait()
		v.state.AssertExpectations(v.T())
		v.ss.AssertExpectations(v.T())
		v.me.AssertExpectations(v.T())
	}
}

// ConcurrencyTestSetup is a sub-test method. It is not invoked independently, rather
// it is executed as part of other test methods. It provides some setups for those test methods.
// On receiving a concurrency degree, and number of consensus nodes, consNum, it generates a mock verifier engine
// and an execution node id. It then mocks a consensus committee for the verifier engine to contact, and mocks
// the submit method of the verifier node. It prepares the verifier engine to accept a valid execution receipt
// from the execution node and emit an empty result approval to all the consensus nodes. It then returns the
// verifier engine and identity of the consensus nodes for more advance caller tests to use.
// The concurrency degree is used to mock the reception of identical execution results
func (v *VerifierEngineTestSuit) ConcurrencyTestSetup(degree, consNum int) (*flow.Identity, *Engine, *flow.ExecutionReceipt) {
	// creating a new engine
	vrfy, err := New(v.e.log, v.net, v.e.state, v.e.me)
	require.Nil(v.T(), err, "could not create an engine")

	// a mock staked execution node for generating a mock execution receipt
	exeID := flow.Identity{
		NodeID:  flow.Identifier{0x02, 0x02, 0x02, 0x02},
		Address: "mock-en-address",
		Role:    flow.RoleExecution,
	}

	// mocking state fo Final().Identity(originID)
	v.state.On("Final").Return(v.ss).Times(degree)
	v.ss.On("Identity", exeID.NodeID).Return(exeID, nil).Times(degree)

	// a set of mock staked nodes
	ids := generateMockIdentities(consNum)
	// extracting mock consensus nodes IDs
	consIDs := ids.Filter(identity.HasRole(flow.RoleConsensus))

	// mocking for Final().Identities(identity.HasRole(flow.RoleConsensus))
	// since all ERs are the same, only one call for identity of consensus nodes should happen
	v.state.On("Final").Return(v.ss).Once()
	v.ss.On("Identities", testifymock.Anything).Return(consIDs, nil)

	// generating a random execution receipt and its corresponding result approval
	er := verification.RandomERGen()
	restApprov := verification.RnadRAGen(er)
	// extracting mock consensus nodes IDs
	params := []interface{}{restApprov}
	for _, targetID := range consIDs {
		params = append(params, targetID.NodeID)
	}

	// the happy path ends by the verifier engine emitting a
	// result approval to ONLY all the consensus nodes
	// since all ERs are the same, only one submission should happen
	v.con.On("Submit", params...).
		Return(nil).
		Once()

	return &exeID, vrfy, er
}

// TestProcessHappyPathConcurrentERs covers the happy path of the verifier engine on concurrently
// receiving a valid execution receipt several times
// The expected behavior is to verify only a single copy of those receipts while dismissing the rest
func (v *VerifierEngineTestSuit) TestProcessHappyPathConcurrentERs() {
	// ConcurrencyDegree defines the number of concurrent identical ER that are submitted to the
	// verifier node
	const ConcurrencyDegree = 10

	// mocks an execution ID and a verifier engine
	// also mocks the reception of 10 concurrent identical execution results
	// as well as a random execution receipt (er) and its mocked execution receipt
	exeID, vrfy, er := v.ConcurrencyTestSetup(ConcurrencyDegree, 100)

	// emitting an execution receipt form the execution node
	errCount := 0
	for i := 0; i < ConcurrencyDegree; i++ {
		err := vrfy.Process(exeID.NodeID, er)
		if err != nil {
			errCount++
		}
	}
	// all ERs are the same, so only one of them should be processed
	assert.Equal(v.T(), errCount, ConcurrencyDegree-1)

	vrfy.wg.Wait()
	v.con.AssertExpectations(v.T())
	v.ss.AssertExpectations(v.T())
	v.state.AssertExpectations(v.T())
}

// TestProcessHappyPathConcurrentERs covers the happy path of the verifier engine on concurrently
// receiving a valid execution receipt several times each over a different threads
// In other words, this test concerns invoking the Process method over threads
// The expected behavior is to verify only a single copy of those receipts while dismissing the rest
func (v *VerifierEngineTestSuit) TestProcessHappyPathConcurrentERsConcurrently() {
	// Todo this test is currently broken as it assumes the Process method of engine to
	// be called sequentially and not over a thread
	// We skip it as it is not required for MVP
	// Skipping this test for now
	v.T().SkipNow()

	// ConcurrencyDegree defines the number of concurrent identical ER that are submitted to the
	// verifier node
	const ConcurrencyDegree = 10

	// mocks an execution ID and a verifier engine
	// also mocks the reception of 10 concurrent identical execution results
	// as well as a random execution receipt (er) and its mocked execution receipt
	exeID, vrfy, er := v.ConcurrencyTestSetup(ConcurrencyDegree, 100)

	// emitting an execution receipt form the execution node
	errCount := 0
	mu := sync.Mutex{}
	for i := 0; i < ConcurrencyDegree; i++ {
		go func() {
			err := vrfy.Process(exeID.NodeID, er)
			if err != nil {
				mu.Lock()
				errCount++
				mu.Unlock()
			}
		}()
	}
	// all ERs are the same, so only one of them should be processed
	assert.Equal(v.T(), errCount, ConcurrencyDegree-1)

	vrfy.wg.Wait()
	v.con.AssertExpectations(v.T())
	v.ss.AssertExpectations(v.T())
	v.state.AssertExpectations(v.T())
}

// TestProcessHappyPathConcurrentDifferentERs covers the happy path of the verifier engine on concurrently
// receiving several valid execution receipts
// The expected behavior is to verify all of them and emit one submission of result approval per input receipt
func (v *VerifierEngineTestSuit) TestProcessHappyPathConcurrentDifferentERs() {
	// ConcurrencyDegree defines the number of concurrent identical ER that are submitted to the
	// verifier node
	const ConcurrencyDegree = 10

	// creating a new engine
	vrfy, err := New(v.e.log, v.net, v.e.state, v.e.me)
	require.Nil(v.T(), err, "could not create an engine")

	// a mock staked execution node for generating a mock execution receipt
	exeID := flow.Identity{
		NodeID:  flow.Identifier{0x02, 0x02, 0x02, 0x02},
		Address: "mock-en-address",
		Role:    flow.RoleExecution,
	}

	// mocking state fo Final().Identity(originID)
	v.state.On("Final").Return(v.ss).Times(ConcurrencyDegree)
	v.ss.On("Identity", exeID.NodeID).Return(exeID, nil).Times(ConcurrencyDegree)

	// a set of mock staked nodes
	ids := generateMockIdentities(100)
	// extracting mock consensus nodes IDs
	consIDs := ids.Filter(identity.HasRole(flow.RoleConsensus))

	// mocking for Final().Identities(identity.HasRole(flow.RoleConsensus))
	// since ERs distinct, distinct calls for retrieving consensus nodes identity should happen
	v.state.On("Final").Return(v.ss).Times(ConcurrencyDegree)
	v.ss.On("Identities", testifymock.Anything).Return(consIDs, nil).Times(ConcurrencyDegree)

	testTable := [ConcurrencyDegree]struct {
		receipt *flow.ExecutionReceipt
		params  []interface{} // parameters of the resulted Submit method of the engine corresponding to receipt
	}{}

	// preparing the test table
	for i := 0; i < ConcurrencyDegree; i++ {
		// generating a random execution receipt and its corresponding result approval
		er := verification.RandomERGen()
		restApprov := verification.RnadRAGen(er)

		params := []interface{}{restApprov}
		// generating mock consensus nodes as recipients of the result approval
		for _, targetID := range consIDs {
			params = append(params, targetID.NodeID)
		}

		testTable[i].receipt = er
		testTable[i].params = params
	}

	// emitting an execution receipt form the execution node
	errCount := 0
	for i := 0; i < ConcurrencyDegree; i++ {
		// the happy path ends by the verifier engine emitting a
		// result approval to ONLY all the consensus nodes
		// since ERs distinct, distinct calls for submission should happen
		v.con.On("Submit", testTable[i].params...).
			Return(nil).
			Once()

		err = vrfy.Process(exeID.NodeID, testTable[i].receipt)
		if err != nil {
			errCount++
		}
	}
	// all ERs are the same, so only one of them should be processed
	assert.Equal(v.T(), errCount, 0)

	vrfy.wg.Wait()
	v.con.AssertExpectations(v.T())
	v.ss.AssertExpectations(v.T())
	v.state.AssertExpectations(v.T())
}

// TestOnExecutionReceiptHappyPath covers the happy path of the verifier engine on receiving an valid execution receipt
// The expected behavior is to verify the receipt and emit a result approval to all consensus nodes
func (v *VerifierEngineTestSuit) TestOnExecutionReceiptHappyPath() {
	// creating a new engine
	vrfy, err := New(v.e.log, v.net, v.e.state, v.e.me)
	require.Nil(v.T(), err, "could not create an engine")

	// a mock staked execution node for generating a mock execution receipt
	exeID := flow.Identity{
		NodeID:  flow.Identifier{0x02, 0x02, 0x02, 0x02},
		Address: "mock-en-address",
		Role:    flow.RoleExecution,
	}

	// mocking state fo Final().Identity(originID)
	v.state.On("Final").Return(v.ss).Once()
	v.ss.On("Identity", exeID.NodeID).Return(exeID, nil).Once()

	// a set of mock staked nodes
	ids := generateMockIdentities(100)
	// extracting mock consensus nodes IDs
	consIds := ids.Filter(identity.HasRole(flow.RoleConsensus))

	// mocking for Final().Identities(identity.HasRole(flow.RoleConsensus))
	v.state.On("Final").Return(v.ss).Once()
	v.ss.On("Identities", testifymock.Anything).Return(consIds, nil)

	// generating a random execution receipt and its corresponding result approval
	er := verification.RandomERGen()
	restApprov := verification.RnadRAGen(er)

	// generating mock consensus nodes as recipients of the result approval
	params := []interface{}{restApprov}
	for _, targetID := range consIds {
		params = append(params, targetID.NodeID)
	}

	// the happy path ends by the verifier engine emitting a
	// result approval to ONLY all the consensus nodes
	v.con.On("Submit", params...).
		Return(nil).
		Once()

	// emitting an execution receipt form the execution node
	err = vrfy.Process(exeID.NodeID, er)
	assert.Nil(v.T(), err, "failed processing execution receipt")

	vrfy.wg.Wait()
	v.con.AssertExpectations(v.T())
	v.ss.AssertExpectations(v.T())
	v.state.AssertExpectations(v.T())
}

// generateMockIdentities generates and returns set of random nodes with different roles
// the distribution of the roles is uniforms but not guaranteed
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
