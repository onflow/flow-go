package main

import (
	"flag"
	"math/rand"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/integration/tests/verification"
	verification2 "github.com/dapperlabs/flow-go/model/verification"
	"github.com/dapperlabs/flow-go/module/mempool/stdmap"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/module/metrics/example"
	"github.com/dapperlabs/flow-go/module/trace"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// main runs a local tracer server on the machine and starts monitoring some metrics for sake of verification, which
// increases result approvals counter and checked chunks counter 100 times each
func main() {
	hp := flag.Bool("happypath", false, "run happy path")
	flag.Parse()

	if *hp {
		happyPathExample()
	} else {
		demo()
	}
}

// happyPathExample captures the metrics on running VerificationHappyPath with
// a single execution receipt of 10 chunks
func happyPathExample() {
	example.WithMetricsServer(func(logger zerolog.Logger) {
		tracer, err := trace.NewTracer(logger, "verification")
		if err != nil {
			panic(err)
		}

		// initiates and starts mempool collector
		// since happy path goes very fast leap timer on collector set to 1 nanosecond.
		mempoolCollector := metrics.NewMempoolCollector(1 * time.Nanosecond)
		<-mempoolCollector.Ready()
		verificationCollector := metrics.NewVerificationCollector(tracer, prometheus.DefaultRegisterer, logger)

		// starts happy path
		t := &testing.T{}
		verification.VerificationHappyPath(t, verificationCollector, mempoolCollector, 1, 10)

		<-mempoolCollector.Done()
	})
}

// demo runs a local tracer server on the machine and starts monitoring some metrics for sake of verification, which
// increases result approvals counter and checked chunks counter 100 times each
func demo() {
	example.WithMetricsServer(func(logger zerolog.Logger) {
		tracer, err := trace.NewTracer(logger, "verification")
		if err != nil {
			panic(err)
		}

		verificationCollector := metrics.NewVerificationCollector(tracer, prometheus.DefaultRegisterer, logger)
		mempoolCollector := metrics.NewMempoolCollector(5 * time.Second)

		// starts periodic launch of mempoolCollector
		<-mempoolCollector.Ready()

		// creates a receipt mempool and registers a metric on its size
		receipts, err := stdmap.NewReceipts(100)
		if err != nil {
			panic(err)
		}
		err = mempoolCollector.Register(metrics.ResourceReceipt, receipts.Size)
		if err != nil {
			panic(err)
		}

		// creates a pending receipt mempool and registers a metric on its size
		pendingReceipts, err := stdmap.NewPendingReceipts(100)
		if err != nil {
			panic(err)
		}
		err = mempoolCollector.Register(metrics.ResourcePendingReceipt, pendingReceipts.Size)
		if err != nil {
			panic(err)
		}

		for i := 0; i < 100; i++ {
			chunkID := unittest.ChunkFixture().ID()
			// finder
			verificationCollector.OnExecutionReceiptReceived()
			verificationCollector.OnExecutionResultSent()

			// match
			verificationCollector.OnExecutionResultReceived()
			verificationCollector.OnVerifiableChunkSent()

			// verifier
			verificationCollector.OnVerifiableChunkReceived()
			verificationCollector.OnChunkVerificationStarted(chunkID)

			// adds a receipt to the pending receipts
			receipt := unittest.ExecutionReceiptFixture()
			pendingReceipts.Add(&verification2.PendingReceipt{
				Receipt:  receipt,
				OriginID: unittest.IdentifierFixture(),
			})
			receipts.Add(receipt)

			// adds a synthetic 1 s delay for verification duration
			time.Sleep(1 * time.Second)
			verificationCollector.OnChunkVerificationFinished(chunkID)
			verificationCollector.OnResultApproval()

			// storage tests
			// making randomized verifiable chunks that capture all storage per chunk
			verificationCollector.LogVerifiableChunkSize(rand.Float64() * 10000.0)
		}
	})
}
