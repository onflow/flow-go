package upgrades

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	adminClient "github.com/onflow/flow-go/integration/client"
	"github.com/onflow/flow-go/integration/testnet"
)

func TestStopAtHeight(t *testing.T) {
	suite.Run(t, new(TestStopAtHeightSuite))
}

type TestStopAtHeightSuite struct {
	Suite
}

type StopAtHeightRequest struct {
	Height uint64 `json:"height"`
	Crash  bool   `json:"crash"`
}

func (s *TestStopAtHeightSuite) TestStopAtHeight() {

	enContainer := s.net.ContainerByID(s.exe1ID)

	serverAddr := fmt.Sprintf("localhost:%s", enContainer.Port(testnet.AdminPort))
	admin := adminClient.NewAdminClient(serverAddr)

	// make sure stop at height admin command is available
	resp, err := admin.RunCommand(context.Background(), "list-commands", struct{}{})
	require.NoError(s.T(), err)
	commandsList, ok := resp.Output.([]interface{})
	s.True(ok)
	s.Contains(commandsList, "stop-at-height")

	// wait for some blocks being finalized
	s.BlockState.WaitForHighestFinalizedProgress(s.T(), 2)

	currentFinalized := s.BlockState.HighestFinalizedHeight()

	// stop in 5 blocks
	stopHeight := currentFinalized + 5

	stopAtHeightRequest := StopAtHeightRequest{
		Height: stopHeight,
		Crash:  true,
	}

	resp, err = admin.RunCommand(
		context.Background(),
		"stop-at-height",
		stopAtHeightRequest,
	)
	s.NoError(err)
	commandResponse, ok := resp.Output.(string)
	s.True(ok)
	s.Equal("ok", commandResponse)

	shouldExecute := s.BlockState.WaitForBlocksByHeight(s.T(), stopHeight-1)
	shouldNotExecute := s.BlockState.WaitForBlocksByHeight(s.T(), stopHeight)

	s.ReceiptState.WaitForReceiptFrom(s.T(), shouldExecute[0].Header.ID(), s.exe1ID)
	s.ReceiptState.WaitForNoReceiptFrom(
		s.T(),
		5*time.Second,
		shouldNotExecute[0].Header.ID(),
		s.exe1ID,
	)

	err = enContainer.WaitForContainerStopped(10 * time.Second)
	s.NoError(err)
}
