package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/metrics"
)

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

func sendMetrics(log zerolog.Logger) {
	metrics, err := metrics.NewCollector(log)
	if err != nil {
		panic(err)
	}

	for i := 0; i < 100; i++ {
		metrics.CollectionsPerBlock(1)
		metrics.CollectionsInFinalizedBlock(3)
		metrics.SealsInFinalizedBlock(3)
		metrics.HotStuffBusyDuration(10)
		metrics.HotStuffIdleDuration(10)
		metrics.StartNewView(uint64(i))
		metrics.NewestKnownQC(uint64(i))

		entityID := make([]byte, 32)
		binary.LittleEndian.PutUint32(entityID, uint32(i/6))

		entity2ID := make([]byte, 32)
		binary.LittleEndian.PutUint32(entity2ID, uint32(i/6+100000))
		if i%6 == 0 {
			metrics.StartCollectionToFinalized(flow.HashToID(entityID))
		} else if i%6 == 3 {
			metrics.FinishCollectionToFinalized(flow.HashToID(entityID))
		}

		if i%5 == 0 {
			metrics.StartBlockToSeal(flow.HashToID(entityID))
		} else if i%6 == 3 {
			metrics.FinishBlockToSeal(flow.HashToID(entityID))
		}

		time.Sleep(1 * time.Second)
	}
}
