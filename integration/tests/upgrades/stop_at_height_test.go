package upgrades

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestStopAtHeight(t *testing.T) {
	suite.Run(t, new(TestStopAtHeightSuite))
}

type TestStopAtHeightSuite struct {
	Suite
}

type AdminCommandListCommands []string

type StopAtHeightRequest struct {
	Height uint64 `json:"height"`
	Crash  bool   `json:"crash"`
}

func (s *TestStopAtHeightSuite) TestStopAtHeight() {
	enContainer := s.net.ContainerByID(s.exe1ID)

	// make sure stop at height admin command is available
	commandsList := AdminCommandListCommands{}
	err := s.SendExecutionAdminCommand(context.Background(), "list-commands", struct{}{}, &commandsList)
	require.NoError(s.T(), err)

	require.Contains(s.T(), commandsList, "stop-at-height")

	// wait for some blocks being finalized
	s.BlockState.WaitForHighestFinalizedProgress(s.T(), 2)

	currentFinalized := s.BlockState.HighestFinalizedHeight()

	// stop in 5 blocks
	stopHeight := currentFinalized + 5

	stopAtHeightRequest := StopAtHeightRequest{
		Height: stopHeight,
		Crash:  true,
	}

	var commandResponse string
	err = s.SendExecutionAdminCommand(context.Background(), "stop-at-height", stopAtHeightRequest, &commandResponse)
	require.NoError(s.T(), err)

	require.Equal(s.T(), "ok", commandResponse)

	shouldExecute := s.BlockState.WaitForBlocksByHeight(s.T(), stopHeight-1)
	shouldNotExecute := s.BlockState.WaitForBlocksByHeight(s.T(), stopHeight)

	s.ReceiptState.WaitForReceiptFrom(s.T(), shouldExecute[0].Header.ID(), s.exe1ID)
	s.ReceiptState.WaitForNoReceiptFrom(s.T(), 5*time.Second, shouldNotExecute[0].Header.ID(), s.exe1ID)

	err = enContainer.WaitForContainerStopped(10 * time.Second)
	require.NoError(s.T(), err)
}
