package consensus_test

import (
	"os"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/consensus"

	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	mockhotstuff "github.com/dapperlabs/flow-go/consensus/hotstuff/mocks"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/model/flow"
	mockmodule "github.com/dapperlabs/flow-go/module/mock"
	mockstorage "github.com/dapperlabs/flow-go/storage/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestHotStuffFollower(t *testing.T) {
	suite.Run(t, new(HotStuffFollowerSuite))
}

type HotStuffFollowerSuite struct {
	suite.Suite

	committee  *mockhotstuff.Committee
	headers    *mockstorage.Headers
	updater    *mockmodule.Finalizer
	verifier   *mockhotstuff.Verifier
	notifier   *mockhotstuff.FinalizationConsumer
	rootHeader *flow.Header
	rootQC     *model.QuorumCertificate
	finalized  *flow.Header
	pending    []*flow.Header
	follower   *hotstuff.FollowerLoop

	mockConsensus          *MockConsensus
	expectedNotifierRecord *ExpectedRecord
	expectedUpdaterRecord  *ExpectedRecord
}

func (s *HotStuffFollowerSuite) SetupTest() {
	identities := unittest.IdentityListFixture(4, unittest.WithRole(flow.RoleConsensus))
	s.mockConsensus = &MockConsensus{identities: identities}
	s.expectedNotifierRecord = NewExpectedRecord()
	s.expectedUpdaterRecord = NewExpectedRecord()

	// mock consensus committee
	s.committee = &mockhotstuff.Committee{}
	s.committee.On("Identities", mock.Anything, mock.Anything).Return(
		func(blockID flow.Identifier, selector flow.IdentityFilter) flow.IdentityList {
			return identities.Filter(selector)
		},
		nil,
	)
	for _, identity := range identities {
		s.committee.On("Identity", mock.Anything, identity.NodeID).Return(identity, nil)
	}
	s.committee.On("LeaderForView", mock.Anything).Return(
		func(view uint64) flow.Identifier { return identities[int(view)%len(identities)].NodeID },
		nil,
	)

	// mock storage headers
	s.headers = &mockstorage.Headers{}

	// mock finalization updater
	s.updater = &mockmodule.Finalizer{}
	s.updater.On("MakePending", mock.Anything).Return(nil).Run(
		func(args mock.Arguments) {
			require.True(s.T(), s.expectedUpdaterRecord.HasNextExpectedRecord(), "no expected record for this call")
			expectedMethodName, expectedBlockID := s.expectedUpdaterRecord.NextExpectedRecord()
			require.Equal(s.T(), expectedMethodName, "MakePending", "unexpected method call")
			blockID := args.Get(0).(flow.Identifier)
			require.Equal(s.T(), expectedBlockID, blockID, "unexpected block ID")
		},
	)

	// mock finalization updater
	s.verifier = &mockhotstuff.Verifier{}
	//s.verifier.On("VerifyVote", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)
	//s.verifier.On("VerifyQC", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)
	s.verifier.On("VerifyVote", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)
	s.verifier.On("VerifyQC", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

	// mock consumer for finalization notifications
	s.notifier = &mockhotstuff.FinalizationConsumer{}
	s.notifier.On("OnBlockIncorporated", mock.Anything).Return().Run(
		func(args mock.Arguments) {
			require.True(s.T(), s.expectedNotifierRecord.HasNextExpectedRecord(), "no expected record for this call")
			expectedMethodName, expectedBlockID := s.expectedNotifierRecord.NextExpectedRecord()
			require.Equal(s.T(), expectedMethodName, "OnBlockIncorporated", "unexpected method call")
			block := args.Get(0).(*model.Block)
			require.Equal(s.T(), expectedBlockID, block.BlockID, "unexpected block ID")
		},
	)

	// root block and QC
	parentID, err := flow.HexStringToIdentifier("aa7693d498e9a087b1cadf5bfe9a1ff07829badc1915c210e482f369f9a00a70")
	require.NoError(s.T(), err)
	s.rootHeader = &flow.Header{
		ParentID:  parentID,
		Timestamp: time.Now().UTC(),
		Height:    21053,
		View:      52078,
	}
	s.rootQC = &model.QuorumCertificate{
		View:      s.rootHeader.View,
		BlockID:   s.rootHeader.ID(),
		SignerIDs: identities.NodeIDs()[:3],
	}

	// we start with the latest finalized block being the root block
	s.finalized = s.rootHeader
	// and no pending (unfinalized) block
	s.pending = []*flow.Header{}
}

func (s *HotStuffFollowerSuite) BeforeTest(suiteName, testName string) {
	s.expectedNotifierRecord.AppendRecord("OnBlockIncorporated", s.rootHeader)

	var err error
	s.follower, err = consensus.NewFollower(
		zerolog.New(os.Stderr),
		s.committee,
		s.headers,
		s.updater,
		s.verifier,
		s.notifier,
		s.rootHeader,
		s.rootQC,
		s.finalized,
		s.pending,
	)
	require.NoError(s.T(), err)

	select {
	case <-s.follower.Ready():
	case <-time.After(time.Second):
		s.T().Error("timeout on waiting for follower start")
	}
}

func (s *HotStuffFollowerSuite) AfterTest(suiteName, testName string) {
	select {
	case <-s.follower.Done():
	case <-time.After(time.Second):
		s.T().Error("timeout on waiting for expected Follower shutdown")
	}
}

// TestFollowerInitialization verifies that the basic test setup with initialization of the Follower works as expected
func (s *HotStuffFollowerSuite) TestFollowerInitialization() {
	// we expect no additional calls to s.updater or s.notifier
	s.requireForExpectedRecords()
}

// TestFollowerProcessBlock verifies that when submitting a single valid block (child root block),
// the Follower reacts with callbacks to s.updater.MakePending or s.notifier.OnBlockIncorporated with this new block
func (s *HotStuffFollowerSuite) TestFollowerProcessBlock() {
	rootBlockView := s.rootHeader.View
	nextBlock := s.mockConsensus.extendBlock(rootBlockView+1, s.rootHeader)

	s.expectedNotifierRecord.AppendRecord("OnBlockIncorporated", nextBlock)
	s.expectedUpdaterRecord.AppendRecord("MakePending", nextBlock)
	s.follower.SubmitProposal(nextBlock, rootBlockView)
	s.requireForExpectedRecords()
}

func (s *HotStuffFollowerSuite) requireForExpectedRecords() {
	select {
	case <-s.expectedNotifierRecord.AllRecordsRecalled():
	case <-time.After(time.Second):
		s.T().Error("timeout on waiting for expected Notifier calls")
	}
	select {
	case <-s.expectedUpdaterRecord.AllRecordsRecalled():
	case <-time.After(time.Second):
		s.T().Error("timeout on waiting for expected Notifier calls")
	}
}

//func (s *HotStuffFollowerSuite) TestFollowerProcessBlock() {
//	fmt.Println("foo")
//	rootBlockView := s.rootHeader.View
//	nextBlock := s.mockConsensus.extendBlock(rootBlockView+1, s.rootHeader)
//
//	s.expectedNotifierRecord.AppendRecord("OnBlockIncorporated", nextBlock)
//	s.expectedUpdaterRecord.AppendRecord("MakePending", nextBlock)
//	s.follower.SubmitProposal(nextBlock, rootBlockView)
//
//	select {
//	case <-s.expectedNotifierRecord.AllRecordsRecalled():
//	case <-time.After(time.Second):
//		s.T().Error("timeout on waiting for expected Notifier calls")
//	}
//}

//func (s *HotStuffFollowerSuite) TestFollowerInitialization() {
//	fmt.Println("foo")
//	parentView := s.rootHeader.View
//	nextBlock := unittest.BlockHeaderWithParentFixture(s.rootHeader)
//	s.follower.SubmitProposal(&nextBlock, parentView)
//
//	//s.T().
//	print("done")
//	require.Equal(s.T(), 100, s.a)
//	//// construct ForkChoice
//	//s.notifier.On("OnQcIncorporated", root.QC).Return().Once()
//	//fc, err := NewNewestForkChoice(fnlzr, notifier)
//	//require.NoError(t, err)
//	//
//	//s.notifier.AssertExpectations(s.T())
//
//}

//func qc(view uint64, id flow.Identifier) *model.QuorumCertificate {
//	return &model.QuorumCertificate{View: view, BlockID: id}
//}
//

type MockConsensus struct {
	identities flow.IdentityList
}

func (mc *MockConsensus) extendBlock(blockView uint64, parent *flow.Header) *flow.Header {
	nextBlock := unittest.BlockHeaderWithParentFixture(parent)
	nextBlock.View = blockView
	nextBlock.ProposerID = mc.identities[int(blockView)%len(mc.identities)].NodeID
	nextBlock.ParentVoterIDs = mc.identities.NodeIDs()
	return &nextBlock
}

type ExpectedRecord struct {
	sync.Mutex
	recalledElements   int
	method             []string
	blockID            []flow.Identifier
	allRecordsRecalled chan struct{} // channel is closed when all records where recalled
}

func NewExpectedRecord() *ExpectedRecord {
	return &ExpectedRecord{
		Mutex:              sync.Mutex{},
		allRecordsRecalled: nil,
	}
}

func (r *ExpectedRecord) AppendRecord(methodName string, block *flow.Header) {
	r.Lock()
	defer r.Unlock()
	r.method = append(r.method, methodName)
	r.blockID = append(r.blockID, block.ID())
}

func (r *ExpectedRecord) HasNextExpectedRecord() bool {
	r.Lock()
	defer r.Unlock()
	return r.recalledElements < len(r.method)
}

func (r *ExpectedRecord) NextExpectedRecord() (string, flow.Identifier) {
	r.Lock()
	defer r.Unlock()
	recalledElements := r.recalledElements
	r.recalledElements++
	if r.allRecordsRecalled != nil {
		if r.recalledElements == len(r.method) {
			close(r.allRecordsRecalled)
		}
	}
	return r.method[recalledElements], r.blockID[recalledElements]
}

func (r *ExpectedRecord) AllRecordsRecalled() <-chan struct{} {
	r.Lock()
	defer r.Unlock()
	r.allRecordsRecalled = make(chan struct{})
	if r.recalledElements == len(r.method) {
		close(r.allRecordsRecalled)
	}
	return r.allRecordsRecalled
}

type finalizerRecord struct {
}

// Finalizer is an autogenerated mock type for the Finalizer type
//type RecordingFinalizer struct {
//	mock.Mock
//}
//
//// MakeFinal provides a mock function with given fields: blockID
//func (_m *Finalizer) MakeFinal(blockID flow.Identifier) error {
//	ret := _m.Called(blockID)
//
//	var r0 error
//	if rf, ok := ret.Get(0).(func(flow.Identifier) error); ok {
//		r0 = rf(blockID)
//	} else {
//		r0 = ret.Error(0)
//	}
//
//	return r0
//}
//
//// MakePending provides a mock function with given fields: blockID
//func (_m *Finalizer) MakePending(blockID flow.Identifier) error {
//	ret := _m.Called(blockID)
//
//	var r0 error
//	if rf, ok := ret.Get(0).(func(flow.Identifier) error); ok {
//		r0 = rf(blockID)
//	} else {
//		r0 = ret.Error(0)
//	}
//
//	return r0
//}
