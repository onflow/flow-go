package main

import (
	"math/rand"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/model/verification"
	"github.com/dapperlabs/flow-go/model/verification/tracker"
	"github.com/dapperlabs/flow-go/module/mempool/stdmap"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/module/metrics/example"
	"github.com/dapperlabs/flow-go/module/trace"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// main runs a local tracer server on the machine and starts monitoring some metrics for sake of verification, which
// increases result approvals counter and checked chunks counter 100 times each
func main() {
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
		authReceipts, err := stdmap.NewReceipts(100)
		if err != nil {
			panic(err)
		}
		err = mempoolCollector.Register(metrics.ResourceReceipt, authReceipts.Size)
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

		// creates an authenticated collection mempool and registers a metric on its size
		authCollections, err := stdmap.NewCollections(100)
		if err != nil {
			panic(err)
		}
		err = mempoolCollector.Register(metrics.ResourceCollection, authCollections.Size)
		if err != nil {
			panic(err)
		}

		// creates a pending collection mempool and registers a metric on its size
		pendingCollections, err := stdmap.NewPendingCollections(100)
		if err != nil {
			panic(err)
		}
		err = mempoolCollector.Register(metrics.ResourcePendingCollection, pendingCollections.Size)
		if err != nil {
			panic(err)
		}

		// creates a chunk data pack mempool and registers a metric on its size
		chunkDataPacks, err := stdmap.NewChunkDataPacks(100)
		if err != nil {
			panic(err)
		}
		err = mempoolCollector.Register(metrics.ResourceChunkDataPack, chunkDataPacks.Size)
		if err != nil {
			panic(err)
		}

		// creates a chunk data pack tracker mempool and registers a metric on its size
		chunkDataPackTrackers, err := stdmap.NewChunkDataPackTrackers(100)
		if err != nil {
			panic(err)
		}
		err = mempoolCollector.Register(metrics.ResourceChunkDataPackTracker, chunkDataPackTrackers.Size)
		if err != nil {
			panic(err)
		}

		for i := 0; i < 100; i++ {
			chunkID := unittest.ChunkFixture().ID()
			verificationCollector.OnResultApproval()
			verificationCollector.OnChunkVerificationStarted(chunkID)

			// adds a collection to pending and authenticated collections mempool
			coll := unittest.CollectionFixture(1)
			pendingCollections.Add(&verification.PendingCollection{
				Collection: &coll,
				OriginID:   unittest.IdentifierFixture(),
			})
			authCollections.Add(&coll)

			// adds a receipt to the pending receipts
			receipt := unittest.ExecutionReceiptFixture()
			pendingReceipts.Add(&verification.PendingReceipt{
				Receipt:  receipt,
				OriginID: unittest.IdentifierFixture(),
			})
			authReceipts.Add(receipt)

			// adds a chunk data pack as well as a chunk tracker
			cdp := unittest.ChunkDataPackFixture(unittest.IdentifierFixture())
			chunkDataPacks.Add(cdp)
			chunkDataPackTrackers.Add(tracker.NewChunkDataPackTracker(unittest.IdentifierFixture(),
				unittest.IdentifierFixture()))

			// adds a synthetic 1 s delay for verification duration
			time.Sleep(1 * time.Second)
			verificationCollector.OnChunkVerificationFinished(chunkID)
			verificationCollector.OnResultApproval()

			// storage tests
			// making randomized verifiable chunks that capture all storage per chunk
			verificationCollector.OnVerifiableChunkSubmitted(rand.Float64() * 10000.0)
		}
	})
}
