package execution

import (
	"context"
	"testing"

	"github.com/onflow/cadence"
	sdk "github.com/onflow/flow-go-sdk"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/dsl"
)

func TestScheduledCallbacks(t *testing.T) {
	suite.Run(t, new(ScheduledCallbacksSuite))
}

type ScheduledCallbacksSuite struct {
	Suite
}

func (s *ScheduledCallbacksSuite) TestGetStatusReturnsNilForNonExistentCallback() {
	// wait for next height finalized (potentially first height)
	currentFinalized := s.BlockState.HighestFinalizedHeight()
	blockA := s.BlockState.WaitForHighestFinalizedProgress(s.T(), currentFinalized)
	s.T().Logf("got blockA height %v ID %v", blockA.Header.Height, blockA.Header.ID())

	// Execute script to call getStatus(id: 0) on the contract
	// The contract should already be deployed on the service account
	chainID := s.net.Root().Header.ChainID
	chain := chainID.Chain()
	serviceAddress := chain.ServiceAddress()

	getStatusScript := dsl.Main{
		Import: dsl.Import{
			Address: sdk.Address(serviceAddress),
			Names:   []string{"FlowCallbackScheduler"},
		},
		ReturnType: "FlowCallbackScheduler.Status?",
		Code:       "return FlowCallbackScheduler.getStatus(id: 0)",
	}

	result, err := s.AccessClient().ExecuteScript(context.Background(), getStatusScript)
	require.NoError(s.T(), err, "could not execute getStatus script")

	// Verify the result is nil (cadence.Optional with nil value)
	optionalResult, ok := result.(cadence.Optional)
	require.True(s.T(), ok, "result should be a cadence.Optional")
	require.Nil(s.T(), optionalResult.Value, "getStatus(0) should return nil for non-existent callback")

	// Wait for a block to be executed to ensure everything is processed
	blockB := s.BlockState.WaitForHighestFinalizedProgress(s.T(), blockA.Header.Height)
	erBlock := s.ReceiptState.WaitForReceiptFrom(s.T(), flow.Identifier(blockB.Header.ID()), s.exe1ID)
	s.T().Logf("got block result ID %v", erBlock.ExecutionResult.BlockID)
}
