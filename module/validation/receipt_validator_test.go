package validation

import (
	"github.com/onflow/flow-go/module"
	mock2 "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"testing"
)

func TestReceiptValidator(t *testing.T) {
	suite.Run(t, new(ReceiptValidationSuite))
}

type ReceiptValidationSuite struct {
	unittest.BaseChainSuite

	receiptValidator module.ReceiptValidator
	verifier         *mock2.Verifier
}

func (s *ReceiptValidationSuite) SetupTest() {
	s.SetupChain()
	s.verifier = &mock2.Verifier{}
	s.receiptValidator = NewReceiptValidator(s.State, s.IndexDB, s.ResultsDB, s.verifier)
}

func (s *ReceiptValidationSuite) TestReceiptValid() {
	originID := s.ExeID
	previousResult := unittest.ExecutionResultFixture(unittest.WithBlock(&s.LatestFinalizedBlock))
	result := unittest.ExecutionResultFixture(
		unittest.WithBlock(&s.UnfinalizedBlock),
		unittest.WithPreviousResult(*previousResult),
	)
	receipt := unittest.ExecutionReceiptFixture(
		unittest.WithExecutorID(originID),
		unittest.WithResult(result),
	)

	s.verifier.On("Verify",
		mock.Anything,
		mock.Anything,
		mock.Anything).Return(true, nil).Once()

	err := s.receiptValidator.Validate(receipt)
	s.Require().NoError(err, "should successfully validate receipt")
}
