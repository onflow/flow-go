package example

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/dapperlabs/flow-go/module/metrics"
)

func WithMetricsServer(f func(logger zerolog.Logger)) {
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	port := 3030
	server := metrics.NewServer(logger, uint(port), true)
	exitSig := make(chan os.Signal, 1)
	signal.Notify(exitSig, os.Interrupt, syscall.SIGTERM)

	<-server.Ready()

	go f(logger)

	fmt.Printf("server is ready, port: %v\n", port)
	fmt.Printf("launch prometheus server: \n" +
		"prometheus --config.file=../flow-go/module/metrics/test/prometheus.yml\n" +
		"then open http://localhost:9090 to monitor the collected metrics\n")

	<-exitSig
	log.Warn().Msg("component startup aborted")
	os.Exit(1)
}
