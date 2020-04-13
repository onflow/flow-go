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

func main() {

	// Go signal notification works by sending `os.Signal`
	// values on a channel. We'll create a channel to
	// receive these notifications (we'll also make one to
	// notify us when the program can exit).
	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)

	// `signal.Notify` registers the given channel to
	// receive notifications of the specified signals.
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	// This goroutine executes a blocking receive for
	// signals. When it gets one it'll print it out
	// and then notify the program that it can finish.
	go func() {
		sig := <-sigs
		fmt.Println()
		fmt.Println(sig)
		done <- true
	}()

	// The program will wait here until it gets the
	// expected signal (as indicated by the goroutine
	// above sending a value on `done`) and then exit.
	fmt.Println("awaiting signal")
	<-done
	fmt.Println("exiting")
}

// Demo runs a local tracer server on the machine and starts monitoring some metrics for sake of demo
func Demo(logger zerolog.Logger, port int, exit chan os.Signal) {
	// creates a tracer server
	server := metrics.NewServer(logger, uint(port))
	<-server.Ready()
	log.Info().Msg(fmt.Sprintf("server is ready, port: %v", port))

	// executes experiment
	go sendMetrics(logger)

	<-exit
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
