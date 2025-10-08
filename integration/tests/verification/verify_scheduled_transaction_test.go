package verification

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	sdk "github.com/onflow/flow-go-sdk"

	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/integration/tests/lib"
	"github.com/onflow/flow-go/model/flow"
)

func TestVerifyScheduledCallback(t *testing.T) {
	suite.Run(t, new(VerifyScheduledTransactionsuite))
}

type VerifyScheduledTransactionsuite struct {
	Suite
}

func (s *VerifyScheduledTransactionsuite) TestVerifyScheduledCallback() {
	sc := systemcontracts.SystemContractsForChain(s.net.Root().HeaderBody.ChainID)

	// Wait for next height finalized (potentially first height)
	currentFinalized := s.BlockState.HighestFinalizedHeight()
	blockA := s.BlockState.WaitForHighestFinalizedProgress(s.T(), currentFinalized)
	s.T().Logf("got blockA height %v ID %v", blockA.HeaderBody.Height, blockA.ID())

	// Deploy the test contract first
	err := lib.DeployScheduledTransactionTestContract(
		s.AccessClient(),
		sdk.Address(sc.FlowCallbackScheduler.Address),
		sdk.Address(sc.FlowToken.Address),
		sdk.Address(sc.FungibleToken.Address),
		sdk.Identifier(s.net.Root().ID()),
	)
	require.NoError(s.T(), err, "could not deploy test contract")

	// Wait for next height finalized before scheduling callback
	s.BlockState.WaitForHighestFinalizedProgress(s.T(), s.BlockState.HighestFinalizedHeight())

	// Schedule a callback for 10 seconds in the future
	scheduleDelta := int64(10)
	futureTimestamp := time.Now().Unix() + scheduleDelta

	s.T().Logf("scheduling callback at timestamp: %v, current timestamp: %v", futureTimestamp, time.Now().Unix())
	callbackID, err := lib.ScheduleTransactionAtTimestamp(
		futureTimestamp,
		s.AccessClient(),
		sdk.Address(sc.FlowCallbackScheduler.Address),
		sdk.Address(sc.FlowToken.Address),
		sdk.Address(sc.FungibleToken.Address),
	)
	require.NoError(s.T(), err, "could not schedule callback transaction")
	s.T().Logf("scheduled callback with ID: %d", callbackID)

	// wait for block that executed the scheduled callbacks to be sealed (plus some buffer)
	var sealedBlock *flow.Block
	require.Eventually(s.T(), func() bool {
		sealed, ok := s.BlockState.HighestSealed()
		require.True(s.T(), ok)
		sealedBlock = sealed
		// sealed timestamp /1000 to drop the ms, and +2 to add some buffer
		return uint64(sealed.Timestamp/1000) > uint64(futureTimestamp+5)
	}, 30*time.Second, 1000*time.Millisecond)

	// make sure callback executed event was emitted
	eventTypeString := fmt.Sprintf("A.%v.FlowTransactionScheduler.Executed", sc.FlowCallbackScheduler.Address)
	events, err := s.AccessClient().GetEventsForHeightRange(context.Background(), eventTypeString, blockA.HeaderBody.Height, sealedBlock.Height)
	require.NoError(s.T(), err)

	eventCount := 0
	for _, event := range events {
		for range event.Events {
			eventCount++
		}
	}

	require.Equal(s.T(), eventCount, 1, "expected 1 callback executed event")
}
