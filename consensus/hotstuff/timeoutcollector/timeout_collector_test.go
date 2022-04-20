package timeoutcollector

import (
	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/model/flow"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"testing"
)

func TestTimeoutCollector(t *testing.T) {
	suite.Run(t, new(TimeoutCollectorTestSuite))
}

type TimeoutCollectorTestSuite struct {
	suite.Suite

	view                   uint64
	notifier               *mocks.Consumer
	processor              *mocks.TimeoutProcessor
	onNewQCDiscoveredState mock.Mock
	onNewTCDiscoveredState mock.Mock
	collector              *TimeoutCollector
}

func (s *TimeoutCollectorTestSuite) SetupTest() {
	s.view = 1000
	s.notifier = &mocks.Consumer{}
	s.processor = &mocks.TimeoutProcessor{}

	s.collector = NewTimeoutCollector(s.view, s.notifier, s.processor, s.onNewQCDiscovered, s.onNewTCDiscovered)
}

// onQCCreated is a special function that registers call in mocked state.
// ATTENTION: don't change name of this function since the same name is used in:
// s.onNewQCDiscoveredState.On("onNewQCDiscovered") statements
func (s *TimeoutCollectorTestSuite) onNewQCDiscovered(qc *flow.QuorumCertificate) {
	s.onNewQCDiscoveredState.Called(qc)
}

// onNewTCDiscovered is a special function that registers call in mocked state.
// ATTENTION: don't change name of this function since the same name is used in:
// s.onNewTCDiscoveredState.On("onNewTCDiscovered") statements
func (s *TimeoutCollectorTestSuite) onNewTCDiscovered(tc *flow.TimeoutCertificate) {
	s.onNewTCDiscoveredState.Called(tc)
}

func (s *TimeoutCollectorTestSuite) Test_View() {
	require.Equal(s.T(), s.view, s.collector.View())
}

func (s *TimeoutCollectorTestSuite) Test_AddTimeoutHappyPath() {

}
