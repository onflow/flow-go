package upgrades

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/onflow/flow-go/model/flow"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"golang.org/x/sync/errgroup"
)

const stopHeight = 50

func TestVNStopAtHeight(t *testing.T) {
	testingSuite := new(TestVNStopAtHeightSuite)
	testingSuite.extraVNs = 19 // 10 VNs total
	testingSuite.VNsFlag = fmt.Sprintf("--stop-at-height=%d", stopHeight)
	suite.Run(t, testingSuite)
}

type TestVNStopAtHeightSuite struct {
	Suite
}

func (s *TestVNStopAtHeightSuite) TestVNStopAtHeight() {

	// wait for some blocks being finalized
	s.BlockState.WaitForHighestFinalizedProgress(s.T(), stopHeight)

	shouldExecute := s.BlockState.WaitForBlocksByHeight(s.T(), stopHeight)

	s.ReceiptState.WaitForReceiptFrom(s.T(), shouldExecute[0].Header.ID(), s.exe1ID)

	vnContainers := s.net.ContainersByRole(flow.RoleVerification)

	// all VNs should stop
	errGrp := errgroup.Group{}

	for _, vnContainer := range vnContainers {
		vnContainer := vnContainer
		errGrp.Go(func() error {
			return vnContainer.WaitForContainerStopped(10 * time.Second)
		})
	}
	err := errGrp.Wait()

	// for debugging containers stopped
	if err != nil {
		wg := sync.WaitGroup{}
		for i, vnContainer := range vnContainers {
			wg.Add(1)
			i := i
			vnContainer := vnContainer
			go func() {
				defer wg.Done()

				err, running := vnContainer.IsRunning(5 * time.Second)
				require.NoError(s.T(), err)
				fmt.Printf("DEBUG VN[%d] running => %t\n", i, running)
			}()
		}
	}
	require.NoError(s.T(), err)

}
