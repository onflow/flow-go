package approvals

import (
	"github.com/stretchr/testify/mock"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	hotmodel "github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/engine"
	storage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestApprovalProcessingCore(t *testing.T) {
	suite.Run(t, new(ApprovalProcessingCoreTestSuite))
}

type ApprovalProcessingCoreTestSuite struct {
	AssignmentCollectorTestSuite

	payloads *storage.Payloads
	core     *approvalProcessingCore
}

func (c *ApprovalProcessingCoreTestSuite) SetupTest() {
	c.AssignmentCollectorTestSuite.SetupTest()

	c.payloads = &storage.Payloads{}
	c.core = NewApprovalProcessingCore(c.payloads, c.State, c.Assigner, c.SigVerifier, c.SealsPL, uint(len(c.AuthorizedVerifiers)))
}

func (c *ApprovalProcessingCoreTestSuite) TestOnBlockFinalized_RejectOutdatedApprovals() {
	approval := unittest.ResultApprovalFixture(unittest.WithApproverID(c.VerID),
		unittest.WithChunk(c.Chunks[0].Index),
		unittest.WithBlockID(c.Block.ID()))
	err := c.core.ProcessApproval(approval)
	require.NoError(c.T(), err)

	hotstuffBlock := hotmodel.Block{
		BlockID: c.Block.ID(),
	}

	seal := unittest.Seal.Fixture(unittest.Seal.WithBlockID(c.Block.ID()),
		unittest.Seal.WithResult(c.IncorporatedResult.Result))
	payload := unittest.PayloadFixture(unittest.WithSeals(seal))

	c.payloads.On("ByBlockID", mock.Anything).Return(&payload, nil).Once()

	c.core.OnFinalizedBlock(&hotstuffBlock)

	err = c.core.ProcessApproval(approval)
	require.Error(c.T(), err)
	require.True(c.T(), engine.IsOutdatedInputError(err))
}
