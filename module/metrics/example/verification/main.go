package main

import (
	"flag"
	"math/rand"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"

	vertestutil "github.com/dapperlabs/flow-go/engine/testutil/verification"
	"github.com/dapperlabs/flow-go/engine/verification/match"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	vermodel "github.com/dapperlabs/flow-go/model/verification"
	"github.com/dapperlabs/flow-go/module/buffer"
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

		// starts happy path
		t := &testing.T{}
		verificationCollector := metrics.NewVerificationCollector(tracer, prometheus.DefaultRegisterer, logger)
		vertestutil.VerificationHappyPath(t, 1, 10, verificationCollector, mempoolCollector)
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

		// creates a pending receipts mempool and registers a metric on its size
		pendingReceipts, err := stdmap.NewReceiptDataPacks(100)
		if err != nil {
			panic(err)
		}
		err = mempoolCollector.Register(metrics.ResourcePendingReceipt, pendingReceipts.Size)
		if err != nil {
			panic(err)
		}

		// creates pending receipt ids by block mempool, and registers size method of backend for metrics
		receiptIDsByBlock, err := stdmap.NewIdentifierMap(100)
		if err != nil {
			panic(err)
		}
		err = mempoolCollector.Register(metrics.ResourcePendingReceiptIDsByBlock, receiptIDsByBlock.Size)
		if err != nil {
			panic(err)
		}

		// creates pending receipt ids by result mempool, and registers size method of backend for metrics
		receiptIDsByResult, err := stdmap.NewIdentifierMap(100)
		if err != nil {
			panic(err)
		}
		err = mempoolCollector.Register(metrics.ResourceReceiptIDsByResult, receiptIDsByResult.Size)
		if err != nil {
			panic(err)
		}

		// creates pending results mempool, and registers size method of backend for metrics
		pendingResults := stdmap.NewPendingResults()
		err = mempoolCollector.Register(metrics.ResourcePendingResult, pendingResults.Size)
		if err != nil {
			panic(err)
		}

		// creates pending chunks mempool, and registers size method of backend for metrics
		pendingChunks := match.NewChunks(100)
		err = mempoolCollector.Register(metrics.ResourcePendingChunk, pendingChunks.Size)
		if err != nil {
			panic(err)
		}

		// creates processed results ids mempool, and registers size method of backend for metrics
		processedResultsIDs, err := stdmap.NewIdentifiers(100)
		if err != nil {
			panic(err)
		}
		err = mempoolCollector.Register(metrics.ResourceProcessedResultID, processedResultsIDs.Size)
		if err != nil {
			panic(err)
		}

		// creates consensus cache for follower engine, and registers size method of backend for metrics
		pendingBlocks := buffer.NewPendingBlocks()
		err = mempoolCollector.Register(metrics.ResourcePendingBlock, pendingBlocks.Size)
		if err != nil {
			panic(err)
		}

		// Over iterations each metric is gone through
		// a probabilistic experiment with probability 0.5
		// to collect or not.
		// This is done to stretch metrics and scatter their pattern
		// for a clear visualization.
		for i := 0; i < 100; i++ {
			chunkID := unittest.ChunkFixture().ID()
			// finder
			if rand.Int()%2 == 0 {
				verificationCollector.OnExecutionReceiptReceived()
			}

			if rand.Int()%2 == 0 {
				verificationCollector.OnExecutionResultSent()
			}

			// match
			if rand.Int()%2 == 0 {
				verificationCollector.OnExecutionResultReceived()
			}
			if rand.Int()%2 == 0 {
				verificationCollector.OnChunkDataPackReceived()
			}
			if rand.Int()%2 == 0 {
				verificationCollector.OnVerifiableChunkSent()
			}

			// verifier
			if rand.Int()%2 == 0 {
				verificationCollector.OnVerifiableChunkReceived()
			}

			verificationCollector.OnChunkVerificationStarted(chunkID)

			// mempools
			// creates and add a receipt
			receipt := unittest.ExecutionReceiptFixture()
			if rand.Int()%2 == 0 {
				receipts.Add(receipt)
			}

			if rand.Int()%2 == 0 {
				pendingReceipts.Add(&vermodel.ReceiptDataPack{
					Receipt:  receipt,
					OriginID: unittest.IdentifierFixture(),
				})
			}

			if rand.Int()%2 == 0 {
				_, err := receiptIDsByBlock.Append(receipt.ExecutionResult.BlockID, receipt.ID())
				if err != nil {
					panic(err)
				}
			}

			if rand.Int()%2 == 0 {
				_, err = receiptIDsByResult.Append(receipt.ExecutionResult.BlockID, receipt.ExecutionResult.ID())
				if err != nil {
					panic(err)
				}
			}

			if rand.Int()%2 == 0 {
				pendingResults.Add(&flow.PendingResult{
					ExecutorID:      receipt.ExecutorID,
					ExecutionResult: &receipt.ExecutionResult,
				})
			}

			if rand.Int()%2 == 0 {
				pendingChunks.Add(&match.ChunkStatus{
					Chunk:             receipt.ExecutionResult.Chunks[0],
					ExecutionResultID: receipt.ExecutionResult.ID(),
					ExecutorID:        receipt.ExecutorID,
					LastAttempt:       time.Time{},
					Attempt:           0,
				})
			}

			if rand.Int()%2 == 0 {
				processedResultsIDs.Add(receipt.ExecutionResult.ID())
			}

			if rand.Int()%2 == 0 {
				block := unittest.BlockFixture()
				pendingBlocks.Add(unittest.IdentifierFixture(), &messages.BlockProposal{
					Header:  block.Header,
					Payload: block.Payload,
				})
			}

			// adds a synthetic 1 s delay for verification duration
			time.Sleep(1 * time.Second)
			verificationCollector.OnChunkVerificationFinished(chunkID)
			if rand.Int()%2 == 0 {
				verificationCollector.OnResultApproval()
			}

			// storage tests
			// making randomized verifiable chunks that capture all storage per chunk
			verificationCollector.LogVerifiableChunkSize(rand.Float64() * 10000.0)
		}
	})
}
