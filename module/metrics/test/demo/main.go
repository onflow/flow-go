package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// main runs a local tracer server on the machine and starts monitoring some metrics for sake of demo
func main() {
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	port := 3030
	server := metrics.NewServer(logger, uint(port))
	exitSig := make(chan os.Signal, 1)
	signal.Notify(exitSig, os.Interrupt, syscall.SIGTERM)

	<-server.Ready()

	go func() {
		sendMetrics(logger)
	}()

	fmt.Printf("server is ready, port: %v\n", port)
	fmt.Printf("launch prometheus server: \n" +
		"prometheus --config.file=../flow-go/module/metrics/test/prometheus.yml\n" +
		"then open http://localhost:9090 to monitor the collected metrics\n")

	<-exitSig
	log.Warn().Msg("component startup aborted")
	os.Exit(1)
}

// sendMetrics increases result approvals counter and checked chunks counter 100 times each
func sendMetrics(log zerolog.Logger) {
	metrics, err := metrics.NewCollector(log)
	if err != nil {
		panic(err)
	}
	for i := 0; i < 100; i++ {
		blockID := unittest.BlockFixture().ID()
		metrics.OnResultApproval(blockID)
		metrics.OnChunkVerificationStated(blockID)
	}
}
