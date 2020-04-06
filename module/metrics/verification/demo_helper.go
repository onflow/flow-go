package verification

import (
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// Demo runs a local tracer server on the machine and starts monitoring some metrics for sake of demo
func Demo(logger zerolog.Logger, port int, exit chan os.Signal) {
	// creates a tracer server
	server := metrics.NewServer(logger, uint(port))
	<-server.Ready()
	log.Info().Msg(fmt.Sprintf("server is ready, port: %v", port))

	// executes experiment
	go sendMetrics()

	select {
	case <-exit:
		log.Warn().Msg("component startup aborted")
		os.Exit(1)
	}
}

// sendMetrics increases result approvals counter and checked chunks counter 100 times each
func sendMetrics() {
	for i := 0; i < 100; i++ {
		blockID := unittest.BlockFixture().ID()
		incResultApprovalCounter(blockID)
		incCheckedChecksCounter(blockID)
	}
}
