package upgrades

import (
	"context"
	"fmt"
	"github.com/onflow/flow-go/integration/testnet"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/onflow/flow-go/model/flow"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"golang.org/x/sync/errgroup"
)

//const stopHeight = 50

func TestVNStopAtHeight(t *testing.T) {
	testingSuite := new(TestVNStopAtHeightSuite)
	testingSuite.extraVNs = 9 // 10 VNs total
	suite.Run(t, testingSuite)
}

type TestVNStopAtHeightSuite struct {
	Suite
}

func (s *TestVNStopAtHeightSuite) TestVNStopAtHeight() {

	// wait for some blocks being finalized and sealed
	s.BlockState.WaitForHighestFinalizedProgress(s.T(), 20)
	s.BlockState.WaitForSealed(s.T(), 20)

	shouldExecute := s.BlockState.WaitForBlocksByHeight(s.T(), 20)
	s.ReceiptState.WaitForReceiptFrom(s.T(), shouldExecute[0].Header.ID(), s.exe1ID)

	vnContainers := s.net.ContainersByRole(flow.RoleVerification)

	// issue admin command to all VNs
	errGrp := errgroup.Group{}

	// make sure all VNs have stop at height command
	for i, vnContainer := range vnContainers {
		i := i
		vnContainer := vnContainer
		errGrp.Go(func() error {
			commandsList := AdminCommandListCommands{}

			err := s.SendAdminCommand(context.Background(), vnContainer.Ports[testnet.VerNodeAdminPort], "list-commands", struct{}{}, &commandsList)
			require.NoError(s.T(), err)
			require.Contains(s.T(), commandsList, "stop-at-height")

			fmt.Printf("VN[%d] commands => %s\n", i, strings.Join(commandsList, ", "))

			return err
		})
	}
	err := errGrp.Wait()
	require.NoError(s.T(), err)

	// issue stop command to all nodes
	currentFinalized := s.BlockState.HighestFinalizedHeight()

	// stop in 30 blocks
	stopHeight := currentFinalized + 30

	stopAtHeightRequest := StopAtHeightRequest{
		Height: stopHeight,
	}

	for i, vnContainer := range vnContainers {
		i := i
		vnContainer := vnContainer
		errGrp.Go(func() error {

			var commandResponse string
			err = s.SendAdminCommand(context.Background(), vnContainer.Ports[testnet.VerNodeAdminPort], "stop-at-height", stopAtHeightRequest, &commandResponse)
			require.NoError(s.T(), err)

			require.Equal(s.T(), "ok", commandResponse)

			fmt.Printf("VN[%d] responded => %s\n", i, commandResponse)

			return err
		})
	}
	err = errGrp.Wait()
	require.NoError(s.T(), err)

	// all VNs should stop
	errGrp = errgroup.Group{}

	for _, vnContainer := range vnContainers {
		vnContainer := vnContainer
		errGrp.Go(func() error {
			return vnContainer.WaitForContainerStopped(15 * time.Second)
		})
	}
	err = errGrp.Wait()

	// for debugging containers stopped
	if err != nil {
		wg := sync.WaitGroup{}
		for i, vnContainer := range vnContainers {
			wg.Add(1)
			i := i
			vnContainer := vnContainer
			go func() {
				defer wg.Done()

				err, running := vnContainer.IsRunning(10 * time.Second)
				fmt.Printf("DEBUG VN[%d] running => %t\n", i, running)
				require.NoError(s.T(), err)
			}()
		}
	}
	// end of debug
	require.NoError(s.T(), err)

	highestSealed, sealed := s.BlockState.HighestSealed()
	require.True(s.T(), sealed)
	// in this case when all VNs stop at the same height, sealing should halt at precisely one block before given stop
	require.Equal(s.T(), uint64(stopHeight)-1, highestSealed.Header.Height)

	//s.log.Info().Msg("")
	fmt.Printf("MAKSIO NODES GOING UP NOW\n")

	// now, start nodes again, and they should resume verification and sealing should continue
	errGrp = errgroup.Group{}

	for _, vnContainer := range vnContainers {
		vnContainer := vnContainer
		errGrp.Go(func() error {
			return vnContainer.Restart()
		})
	}
	err = errGrp.Wait()
	require.NoError(s.T(), err)

	// wait for sealing to pickup
	s.BlockState.WaitForSealed(s.T(), stopHeight+20)

}
