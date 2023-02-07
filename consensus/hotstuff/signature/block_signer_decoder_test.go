package signature

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	hotstuff "github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/order"
	"github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/state"
	"github.com/onflow/flow-go/storage"
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
	s.committee.On("IdentitiesByBlock", mock.Anything).Return(s.allConsensus, nil).Maybe()

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

// Test_UnknownBlock tests handling of an unknwon block.
// At the moment, hotstuff.Committee returns an storage.ErrNotFound for an unknown block,
// which we expect the `BlockSignerDecoder` to wrap into a `state.UnknownBlockError`
func (s *blockSignerDecoderSuite) Test_UnknownBlock() {
	*s.committee = *hotstuff.NewDynamicCommittee(s.T())
	s.committee.On("IdentitiesByBlock", mock.Anything).Return(nil, storage.ErrNotFound)

	ids, err := s.decoder.DecodeSignerIDs(s.block.Header)
	require.Empty(s.T(), ids)
	require.True(s.T(), state.IsUnknownBlockError(err))
}

// Test_UnexpectedCommitteeException verifies that `BlockSignerDecoder`
// does _not_ erroneously interpret an unexpecgted exception from the committee as
// a sign of an unknown block, i.e. the decouder should _not_ return an `state.UnknownBlockError`
func (s *blockSignerDecoderSuite) Test_UnexpectedCommitteeException() {
	exception := errors.New("unexpected exception")
	*s.committee = *hotstuff.NewDynamicCommittee(s.T())
	s.committee.On("IdentitiesByBlock", mock.Anything).Return(nil, exception)

	ids, err := s.decoder.DecodeSignerIDs(s.block.Header)
	require.Empty(s.T(), ids)
	require.False(s.T(), state.IsUnknownBlockError(err))
	require.True(s.T(), errors.Is(err, exception))
}

// Test_InvalidIndices verifies that `BlockSignerDecoder` returns
// signature.InvalidSignerIndicesError is the signer indices in the provided header
// are not a valid encoding.
func (s *blockSignerDecoderSuite) Test_InvalidIndices() {
	s.block.Header.ParentVoterIndices = unittest.RandomBytes(1)
	ids, err := s.decoder.DecodeSignerIDs(s.block.Header)
	require.Empty(s.T(), ids)
	require.True(s.T(), signature.IsInvalidSignerIndicesError(err))
}
