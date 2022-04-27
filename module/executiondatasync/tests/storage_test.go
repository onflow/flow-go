package tracker

import (
	"context"
	"sync"
	"testing"

	"github.com/ipfs/go-cid"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/module/state_synchronization"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
)

// TODO: write a concurrency test with a bunch of threads creating status trackers and
// everything and running concurrently, and also a separate thread that's pruning,
// and in the very end still check that everything is consistent.
// https://pkg.go.dev/sync/atomic#AddUint64
// run this multiple times in a loop
// let the loop variables dictate number of concurrent trackers, and total number of heights

type storageConcurrencyTest struct {
	nextHeightToRequest *atomic.Uint64
	storage             *state_synchronization.Storage
	maxHeight           uint64
	// TODO: add all blobstore stuff at the very beginning
	blobs  map[uint64][]blobs.Blob
	bstore blockstore.Blockstore
	start  chan struct{}
}

func (sct *storageConcurrencyTest) setup(maxHeight uint64) {
	sct.maxHeight = maxHeight
	sct.nextHeightToRequest = atomic.NewUint64(0)
	sct.blobs = make(map[uint64][]blobs.Blob)
	sct.start = make(chan struct{})

}

func (sct *storageConcurrencyTest) worker(wg *sync.WaitGroup, t *testing.T, storage *state_synchronization.Storage) {
	defer wg.Done()

	<-sct.start

	var h uint64
	for h <= sct.maxHeight {
		h = sct.nextHeightToRequest.Inc()

		blockID := unittest.IdentifierFixture()
		// TODO: update this to be the ID of the root blob
		rootID := unittest.IdentifierFixture()

		tracker := storage.GetStatusTracker(blockID, h, rootID)
		require.NoError(t, tracker.StartTransfer())

		blobs := sct.blobs[h]
		sct.bstore.PutMany(context.Background(), blobs)

		cids := make([]cid.Cid, len(blobs))
		for i, blob := range blobs {
			cids[i] = blob.Cid()
		}
		require.NoError(t, tracker.TrackBlobs(cids))

		_, err := tracker.FinishTransfer()
		require.NoError(t, err)
	}

	// TODO: prune at the very end of the test and check if all the cids are deleted
	// Then call Check, localFinalizedHeight, and StoredDataLowerBound again
}

func TestStorageConcurrency(t *testing.T) {
}

// TODO: write a basic test to check that Prune actually deletes everything we expect

// TODO: edge case: need to be able to handle expiry of a height
// while some request is still in progress. Or, if we only purge
// incorporated heights, then this could never happen.

// TODO: set bloom filter threshold

// TODO: setup value threshold, because all values are small.
// TODO: setup value log GC on all DB's including blob store for execution node.
// TODO: including blobstore for both!!n https://pkg.go.dev/github.com/ipfs/go-ds-badger#Datastore.CollectGarbage

// TODO: test a shutdown and restart of DB.
// Test a restart right after purging to the limit, ie where
// storeddatalb is equal to latestincorporatedheight, and there are
// no blob tree records. This should capture the case where
// latestBlobTreeRecordHeight == 0

// TODO: test a case where there is a gap between blob tree record heights *past the incorporated height*
// Currently, this should *fail*, because there's a bug in the code. Then, once we have this, we can fix it.

// --------------------------------------------------------------------------------

// Have a single worker processing both receipts and finalized blocks (seals)
// This worker updates the map of currently known requests, and sends them via
// a channel to fetch workers
// when a block is sealed, we can cancel all requests for ED at the same height
// which aren't the sealed one
// when we receive a receipt for which it's already sealed, we can skip requesting
// it.

// maybe there is a separate Fulfiller routine
// It listens for sealed block notifications from the receipt processor routine
// it also listens for request fulfilled nofications from fetch workers
// Using this, it can perform fulfillment (and also notification).
// I think it is safe for the fulfiller to run at the same time as the pruner,
// because the fulfilled height only ever increases.
// TODO: make sure there are no memory leaks

// Now, where do writes to the DB come in?
// The fulfiller should write the latest fulfilled height to the DB

// On restart:
// starting from the latestFullfilledHeight, we first send all sealed ED ids
// up to the sealed height to the fulfiller.
// We also also queue requests for all of these heights
// Then, potentially we check all heights greater than the sealed height,
// and requeue all requests for these.
// A simplified version of this:
// Simply first queue requests for all tracked ED ids past the latestFullfilledHeight
// Then, when we get our first finalization notification, we can simply send all seal
// notifications starting from latestFullfilledHeight to the fulfiller.
// Or, we send all seal notification from the latestFullfilledHeight to the sealed height first.
// We should track the latestFulfilledHeight as a metric

// As for cache:
// One option is we simply don't consider this, ie we assume we will never be too far behind
// Another option, is we let the fulfiller decide how many ExecutionData to keep in memory.
// For example, it can limit how many Execution Data to keep in memory, and once we've reached
// the limit, it can start setting ExecutionData field to nil.
// It is also capable of deleting all non-matching items in memory as soon as it receives
// the sealed notification for each height
// The benefit of simply not considering this, is that the fulfiller actually doesn't need to have
// a reference to the blob store at all.

// https://stackoverflow.com/questions/28432658/does-go-garbage-collect-parts-of-slices/28432812#28432812
// https://medium.com/capital-one-tech/building-an-unbounded-channel-in-go-789e175cd2cd

// storage:
// keep track of latest "fulfilled" height: latest height such that
// everything <= that height have been both sealed and downloaded.
// This is where we resume from when we restart.

// Adder and Getter can take in an instance of Synchronizer, and we can use that to synchronize

// TODO: for the hander, we should test all kinds of various edge cases
// E.g duplicate CID's in the same level, cid not found in the blobservice, etc

// TODO: requester components should wait for each other to be ready
// Note: The subscription just needs to be setup before HotStuff is constructed
