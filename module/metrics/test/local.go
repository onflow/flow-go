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
	collector, err := metrics.NewCollector(log)
	if err != nil {
		panic(err)
	}

	for i := 0; i < 100; i++ {
		collector.CollectionsPerBlock(1)
		collector.CollectionsInFinalizedBlock(3)
		collector.SealsInFinalizedBlock(3)
		collector.HotStuffBusyDuration(10, metrics.HotstuffEventTypeTimeout)
		collector.HotStuffWaitDuration(10, metrics.HotstuffEventTypeTimeout)
		collector.HotStuffIdleDuration(10)
		collector.StartNewView(uint64(i))
		collector.NewestKnownQC(uint64(i))

		entityID := make([]byte, 32)
		binary.LittleEndian.PutUint32(entityID, uint32(i/6))

		entity2ID := make([]byte, 32)
		binary.LittleEndian.PutUint32(entity2ID, uint32(i/6+100000))
		if i%6 == 0 {
			collector.StartCollectionToFinalized(flow.HashToID(entityID))
		} else if i%6 == 3 {
			collector.FinishCollectionToFinalized(flow.HashToID(entityID))
		}

		if i%5 == 0 {
			collector.StartBlockToSeal(flow.HashToID(entityID))
		} else if i%6 == 3 {
			collector.FinishBlockToSeal(flow.HashToID(entityID))
		}

		time.Sleep(1 * time.Second)
	}
}
