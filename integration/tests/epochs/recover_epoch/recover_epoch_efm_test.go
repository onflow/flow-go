package recover_epoch

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
)

func TestRecoverEpoch(t *testing.T) {
	suite.Run(t, new(RecoverEpochSuite))
}

type RecoverEpochSuite struct {
	Suite
}

// TestRecoverEpoch ensures that the recover_epoch transaction flow works as expected. This test will simulate the network going
// into EFM by taking a consensus node offline before completing the DKG. While in EFM mode the test will execute the efm-recover-tx-args
// CLI command to generate transaction arguments to submit a recover_epoch transaction, after submitting the transaction the test will
// ensure the network is healthy.
func (s *RecoverEpochSuite) TestRecoverEpoch() {
	s.AwaitEpochPhase(context.Background(), 0, flow.EpochPhaseSetup, 20*time.Second, time.Second)
	fmt.Println("in epoch phase setup")

	sns := s.GetContainersByRole(flow.RoleConsensus)
	_ = sns[0].Pause()

	// @TODO: trigger EFM manually
}
