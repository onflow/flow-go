package signature

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	hotstuff "github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/order"
	"github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestBlockSignerDecoder(t *testing.T) {
	suite.Run(t, new(blockSignerDecoderSuite))
}

type blockSignerDecoderSuite struct {
	suite.Suite
	allConsensus flow.IdentityList
	committee    *hotstuff.DynamicCommittee

	decoder *BlockSignerDecoder
	block   flow.Block
}

func (s *blockSignerDecoderSuite) SetupTest() {
	// the default header fixture creates signerIDs for a committee of 10 nodes, so we prepare a committee same as that
	s.allConsensus = unittest.IdentityListFixture(40, unittest.WithRole(flow.RoleConsensus)).Sort(order.Canonical)

	// mock consensus committee
	s.committee = hotstuff.NewDynamicCommittee(s.T())
	s.committee.On("IdentitiesByEpoch", mock.Anything).Return(s.allConsensus, nil).Maybe()

	// prepare valid test block:
	voterIndices, err := signature.EncodeSignersToIndices(s.allConsensus.NodeIDs(), s.allConsensus.NodeIDs())
	require.NoError(s.T(), err)
	s.block = unittest.BlockFixture()
	s.block.Header.ParentVoterIndices = voterIndices

	s.decoder = NewBlockSignerDecoder(s.committee)
}

// Test_SuccessfulDecode tests happy path decoding
func (s *blockSignerDecoderSuite) Test_SuccessfulDecode() {
	ids, err := s.decoder.DecodeSignerIDs(s.block.Header)
	require.NoError(s.T(), err)
	require.Equal(s.T(), s.allConsensus.NodeIDs(), ids)
}

// Test_RootBlock tests decoder accepts root block with empty signer indices
func (s *blockSignerDecoderSuite) Test_RootBlock() {
	s.block.Header.ParentVoterIndices = nil
	s.block.Header.ParentVoterSigData = nil
	s.block.Header.View = 0

	ids, err := s.decoder.DecodeSignerIDs(s.block.Header)
	require.NoError(s.T(), err)
	require.Empty(s.T(), ids)
}

// Test_UnexpectedCommitteeException verifies that `BlockSignerDecoder`
// does _not_ erroneously interpret an unexpected exception from the committee as
// a sign of an unknown block, i.e. the decoder should _not_ return an `model.ErrViewForUnknownEpoch` or `signature.InvalidSignerIndicesError`
func (s *blockSignerDecoderSuite) Test_UnexpectedCommitteeException() {
	exception := errors.New("unexpected exception")
	*s.committee = *hotstuff.NewDynamicCommittee(s.T())
	s.committee.On("IdentitiesByEpoch", mock.Anything).Return(nil, exception)

	ids, err := s.decoder.DecodeSignerIDs(s.block.Header)
	require.Empty(s.T(), ids)
	require.NotErrorIs(s.T(), err, model.ErrViewForUnknownEpoch)
	require.False(s.T(), signature.IsInvalidSignerIndicesError(err))
	require.True(s.T(), errors.Is(err, exception))
}

// Test_UnknownEpoch tests handling of a block from an unknown epoch.
// It should propagate the sentinel error model.ErrViewForUnknownEpoch from Committee.
func (s *blockSignerDecoderSuite) Test_UnknownEpoch() {
	*s.committee = *hotstuff.NewDynamicCommittee(s.T())
	s.committee.On("IdentitiesByEpoch", mock.Anything).Return(nil, model.ErrViewForUnknownEpoch)

	ids, err := s.decoder.DecodeSignerIDs(s.block.Header)
	require.Empty(s.T(), ids)
	require.ErrorIs(s.T(), err, model.ErrViewForUnknownEpoch)
}

// Test_InvalidIndices verifies that `BlockSignerDecoder` returns
// signature.InvalidSignerIndicesError if the signer indices in the provided header
// are not a valid encoding.
func (s *blockSignerDecoderSuite) Test_InvalidIndices() {
	s.block.Header.ParentVoterIndices = unittest.RandomBytes(1)
	ids, err := s.decoder.DecodeSignerIDs(s.block.Header)
	require.Empty(s.T(), ids)
	require.True(s.T(), signature.IsInvalidSignerIndicesError(err))
}

// Test_EpochTransition verifies that `BlockSignerDecoder` correctly handles blocks
// near a boundary where the committee changes - an epoch transition.
func (s *blockSignerDecoderSuite) Test_EpochTransition() {
	// The block under test B is the first block of a new epoch, where the committee changed.
	// B contains a QC formed during the view of B's parent -- hence B's signatures must
	// be decoded w.r.t. the committee as of the parent's view.
	//
	//   Epoch 1     Epoch 2
	//   PARENT <- | -- B
	blockView := s.block.Header.View
	parentView := s.block.Header.ParentView
	epoch1Committee := s.allConsensus
	epoch2Committee, err := s.allConsensus.SamplePct(.8)
	require.NoError(s.T(), err)

	*s.committee = *hotstuff.NewDynamicCommittee(s.T())
	s.committee.On("IdentitiesByEpoch", parentView).Return(epoch1Committee, nil).Maybe()
	s.committee.On("IdentitiesByEpoch", blockView).Return(epoch2Committee, nil).Maybe()

	ids, err := s.decoder.DecodeSignerIDs(s.block.Header)
	require.NoError(s.T(), err)
	require.Equal(s.T(), epoch1Committee.NodeIDs(), ids)
}
