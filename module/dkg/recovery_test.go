package dkg

import (
	mockmodule "github.com/onflow/flow-go/module/mock"
	mockprotocol "github.com/onflow/flow-go/state/protocol/mock"
	mockstorage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"testing"
)

func TestBeaconKeyRecovery(t *testing.T) {
	suite.Run(t, new(BeaconKeyRecoverySuite))
}

type BeaconKeyRecoverySuite struct {
	suite.Suite
	local    *mockmodule.Local
	state    *mockprotocol.State
	dkgState *mockstorage.EpochRecoveryMyBeaconKey
}

func (s *BeaconKeyRecoverySuite) SetupTest() {
	s.local = mockmodule.NewLocal(s.T())
	s.state = mockprotocol.NewState(s.T())
	s.dkgState = mockstorage.NewEpochRecoveryMyBeaconKey(s.T())
}

func (s *BeaconKeyRecoverySuite) TestNewBeaconKeyRecovery() {
	recovery, err := NewBeaconKeyRecovery(unittest.Logger(), s.local, s.state, s.dkgState)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), recovery)
}
