package main

import (
	"flag"
	"math/rand"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/verification/match"
	vertestutils "github.com/onflow/flow-go/engine/verification/utils/unittest"
	"github.com/onflow/flow-go/model/messages"
	vermodel "github.com/onflow/flow-go/model/verification"
	"github.com/onflow/flow-go/module/buffer"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/metrics/example"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/utils/unittest"
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

// happyPathExample captures the metrics on running VerificationHappyPath with 10 blocks, each with 10 execution receipts of 10 chunks.
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
		verificationCollector := metrics.NewVerificationCollector(tracer, prometheus.DefaultRegisterer)

		ops := []vertestutils.CompleteExecutionReceiptBuilderOpt{
			vertestutils.WithResults(10),
			vertestutils.WithChunksCount(10),
			vertestutils.WithCopies(1),
		}
		vertestutils.NewVerificationHappyPathTest(t, true, 10, 1, verificationCollector, mempoolCollector, ops...)
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

		vc := metrics.NewVerificationCollector(tracer, prometheus.DefaultRegisterer)
		mc := metrics.NewMempoolCollector(5 * time.Second)

		// starts periodic launch of mempoolCollector
		<-mc.Ready()

		// creates a receipt mempool and registers a metric on its size
		receipts, err := stdmap.NewReceipts(100)
		if err != nil {
			panic(err)
		}
		err = mc.Register(metrics.ResourceReceipt, receipts.Size)
		if err != nil {
			panic(err)
		}

		// creates a pending receipts mempool and registers a metric on its size
		pendingReceipts, err := stdmap.NewReceiptDataPacks(100)
		if err != nil {
			panic(err)
		}
		err = mc.Register(metrics.ResourcePendingReceipt, pendingReceipts.Size)
		if err != nil {
			panic(err)
		}

		// creates pending receipt ids by block mempool, and registers size method of backend for metrics
		receiptIDsByBlock, err := stdmap.NewIdentifierMap(100)
		if err != nil {
			panic(err)
		}
		err = mc.Register(metrics.ResourcePendingReceiptIDsByBlock, receiptIDsByBlock.Size)
		if err != nil {
			panic(err)
		}

		// creates pending receipt ids by result mempool, and registers size method of backend for metrics
		receiptIDsByResult, err := stdmap.NewIdentifierMap(100)
		if err != nil {
			panic(err)
		}
		err = mc.Register(metrics.ResourceReceiptIDsByResult, receiptIDsByResult.Size)
		if err != nil {
			panic(err)
		}

		// creates pending results mempool, and registers size method of backend for metrics
		pendingResults := stdmap.NewResultDataPacks(100)
		err = mc.Register(metrics.ResourcePendingResult, pendingResults.Size)
		if err != nil {
			panic(err)
		}

		// creates pending chunks mempool, and registers size method of backend for metrics
		pendingChunks := match.NewChunks(100)
		err = mc.Register(metrics.ResourcePendingChunk, pendingChunks.Size)
		if err != nil {
			panic(err)
		}

		// creates processed results ids mempool, and registers size method of backend for metrics
		processedResultsIDs, err := stdmap.NewIdentifiers(100)
		if err != nil {
			panic(err)
		}
		err = mc.Register(metrics.ResourceProcessedResultID, processedResultsIDs.Size)
		if err != nil {
			panic(err)
		}

		// creates consensus cache for follower engine, and registers size method of backend for metrics
		pendingBlocks := buffer.NewPendingBlocks()
		err = mc.Register(metrics.ResourcePendingBlock, pendingBlocks.Size)
		if err != nil {
			panic(err)
		}

		// Over iterations each metric is gone through
		// a probabilistic experiment with probability 0.5
		// to collect or not.
		// This is done to stretch metrics and scatter their pattern
		// for a clear visualization.
		for i := 0; i < 100; i++ {
			// assigner
			tryRandomCall(vc.OnExecutionReceiptReceived)
			tryRandomCall(func() {
				vc.OnChunksAssignmentDoneAtAssigner(rand.Int() % 10)
			})
			tryRandomCall(vc.OnAssignedChunkProcessedAtAssigner)
			tryRandomCall(func() {
				vc.OnFinalizedBlockArrivesAtAssigner(uint64(i))
			})

			// finder
			tryRandomCall(vc.OnExecutionReceiptReceived)
			tryRandomCall(vc.OnExecutionResultSent)

			// match
			tryRandomCall(vc.OnExecutionResultReceived)
			tryRandomCall(vc.OnChunkDataPackReceived)
			tryRandomCall(vc.OnVerifiableChunkSent)

			// verifier
			tryRandomCall(vc.OnVerifiableChunkReceivedAtVerifierEngine)

			// memory pools
			receipt := unittest.ExecutionReceiptFixture()
			tryRandomCall(func() {
				receipts.Add(receipt)
			})

			tryRandomCall(func() {
				pendingReceipts.Add(&vermodel.ReceiptDataPack{
					Receipt:  receipt,
					OriginID: unittest.IdentifierFixture(),
				})
			})

			tryRandomCall(func() {
				err := receiptIDsByBlock.Append(receipt.ExecutionResult.BlockID, receipt.ID())
				if err != nil {
					panic(err)
				}
			})

			tryRandomCall(func() {
				err = receiptIDsByResult.Append(receipt.ExecutionResult.BlockID, receipt.ExecutionResult.ID())
				if err != nil {
					panic(err)
				}
			})

			tryRandomCall(func() {
				pendingResults.Add(&vermodel.ResultDataPack{
					ExecutorID:      receipt.ExecutorID,
					ExecutionResult: &receipt.ExecutionResult,
				})
			})

			tryRandomCall(func() {
				pendingChunks.Add(&match.ChunkStatus{
					Chunk:             receipt.ExecutionResult.Chunks[0],
					ExecutionResultID: receipt.ExecutionResult.ID(),
					ExecutorID:        receipt.ExecutorID,
					LastAttempt:       time.Time{},
					Attempt:           0,
				})
			})

			tryRandomCall(func() {
				processedResultsIDs.Add(receipt.ExecutionResult.ID())
			})

			tryRandomCall(func() {
				block := unittest.BlockFixture()
				pendingBlocks.Add(unittest.IdentifierFixture(), &messages.BlockProposal{
					Header:  block.Header,
					Payload: block.Payload,
				})
			})

			// adds a synthetic 1 s delay for verification duration
			time.Sleep(1 * time.Second)

			tryRandomCall(vc.OnResultApprovalDispatchedInNetwork)
		}
	})
}

// tryRandomCall executes function f with a probability of 1/2 that does not necessarily follow a uniform distribution.
//
// DISCLAIMER: this function should not be utilized for production code. Solely meant for testing and demo.
func tryRandomCall(f func()) {
	if rand.Int()%2 == 0 {
		f()
	}
}
