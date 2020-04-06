package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/module/metrics/verification"
)

func main() {
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	port := 3030

	// to notify the s
	exitSig := make(chan os.Signal, 1)
	signal.Notify(exitSig, os.Interrupt, syscall.SIGTERM)
	verification.Demo(logger, port, exitSig)
}
