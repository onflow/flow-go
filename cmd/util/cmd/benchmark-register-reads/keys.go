package benchmark_register_reads

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand/v2"
	"path/filepath"
	"strings"

	dshelp "github.com/ipfs/boxo/datastore/dshelp"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	pebbleds "github.com/ipfs/go-ds-pebble"
	mh "github.com/multiformats/go-multihash"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
)

// maxRootBlobSize is the size up to which a blob of an execution data datastore is read as the
// root blob of a block's execution data. The root blob holds the block ID and the IDs of the
// chunk blobs, so it is small, while the chunk blobs hold the execution data of one collection
// each.
const maxRootBlobSize = 64 << 10

// checkpointLeafNodeBatchBufferSize is the buffer size, in batches of [wal.LeafNodeBatchSize] leaf
// nodes, of the channel the checkpoint readers push to.
const checkpointLeafNodeBatchBufferSize = 64

// registerKey is a register to read, as both the register ID (used by the register store) and the
// trie path (used by the merkle trie).
type registerKey struct {
	id   flow.RegisterID
	path ledger.Path
}

// sampleCheckpointKeys reads up to count leaf nodes of the given single-trie checkpoint and returns
// their registers. The checkpoint's leaf nodes are read in trie path order, which is unrelated to
// the register key order, so the first count leaf nodes are a uniform sample of the checkpoint's
// registers.
//
// Expected error returns during normal operations:
//   - [context.Canceled], [context.DeadlineExceeded]: if the context is cancelled
func sampleCheckpointKeys(
	ctx context.Context,
	checkpointFile string,
	expectedRootHash ledger.RootHash,
	count int,
	workerCount int,
	log zerolog.Logger,
) ([]registerKey, error) {
	checkpointDir, checkpointFileName := filepath.Split(checkpointFile)

	cct, cancel := context.WithCancel(ctx)
	defer cancel()

	leafNodeBatches := make(chan []*wal.LeafNode, checkpointLeafNodeBatchBufferSize)
	readErrCh := make(chan error, 1)
	go func() {
		readErrCh <- wal.OpenAndReadLeafNodesFromCheckpointV6Concurrently(
			cct, leafNodeBatches, checkpointDir, checkpointFileName, expectedRootHash, workerCount, log)
	}()

	keys := make([]registerKey, 0, count)
	complete := false
	for batch := range leafNodeBatches {
		if complete {
			// keep draining the channel so that the readers can finish; the sample is complete
			continue
		}
		for _, leafNode := range batch {
			key, err := leafNode.Payload.Key()
			if err != nil {
				return nil, fmt.Errorf("could not get key from register payload: %w", err)
			}
			registerID, err := convert.LedgerKeyToRegisterID(key)
			if err != nil {
				return nil, fmt.Errorf("could not get register ID from key: %w", err)
			}

			keys = append(keys, registerKey{id: registerID, path: leafNode.Path})
			if len(keys) == count {
				// the sample is complete, stop reading (the readers stop on the next batch)
				complete = true
				cancel()
				break
			}
		}
	}

	if err := <-readErrCh; err != nil && !errors.Is(err, context.Canceled) {
		return nil, fmt.Errorf("could not read checkpoint file %s: %w", checkpointFileName, err)
	}

	if len(keys) < count {
		return nil, fmt.Errorf("checkpoint %s has %d registers, which is less than the %d keys to sample",
			checkpointFileName, len(keys), count)
	}
	return keys, nil
}

// missingKeys returns count registers that do not exist in the state, with random owners and keys.
// The trie path of a missing register is derived the same way the state derives it, so that the
// mtrie can be asked for it too.
func missingKeys(count int, seed int64, pathFinderVersion uint8) ([]registerKey, error) {
	random := rand.New(rand.NewPCG(uint64(seed), uint64(seed)+1)) //nolint:gosec // benchmark keys

	keys := make([]registerKey, 0, count)
	for i := range count {
		owner := make([]byte, flow.AddressLength)
		for offset := 0; offset < len(owner); offset += 8 {
			binary.BigEndian.PutUint64(owner[offset:], random.Uint64())
		}

		registerID := flow.RegisterID{Owner: string(owner), Key: fmt.Sprintf("benchmark missing key %d", i)}
		path, err := pathfinder.KeyToPath(convert.RegisterIDToLedgerKey(registerID), pathFinderVersion)
		if err != nil {
			return nil, fmt.Errorf("could not compute path of missing register: %w", err)
		}
		keys = append(keys, registerKey{id: registerID, path: path})
	}
	return keys, nil
}

// keyStream returns count keys to read, drawn uniformly from the given keys, with roughly
// missPercent percent of the reads on registers that do not exist in the state.
func keyStream(
	keys []registerKey,
	count int,
	missPercent int,
	seed int64,
	pathFinderVersion uint8,
) ([]registerKey, error) {
	missingCount := count * missPercent / 100
	missing, err := missingKeys(missingCount, seed, pathFinderVersion)
	if err != nil {
		return nil, err
	}

	random := rand.New(rand.NewPCG(uint64(seed)+2, uint64(seed)+3)) //nolint:gosec // benchmark keys
	stream := make([]registerKey, 0, count)
	nextMissing := 0
	for i := range count {
		if missingCount > 0 && i%100 < missPercent {
			stream = append(stream, missing[nextMissing%missingCount])
			nextMissing++
			continue
		}
		stream = append(stream, keys[random.IntN(len(keys))])
	}
	return stream, nil
}

// replayKeys reads the execution data of up to blockCount blocks from the given execution data
// datastore and returns the registers each block touched, in the order of the execution data
// datastore.
//
// The registers of a block are the ones its trie update touches, which is what the execution
// reads from the state (minus the registers the block reads but never writes, and with the
// registers that two of its collections touch read twice).
//
// Expected error returns during normal operations:
//   - [context.Canceled], [context.DeadlineExceeded]: if the context is cancelled
func replayKeys(
	ctx context.Context,
	executionDataDir string,
	blockCount int,
	log zerolog.Logger,
) ([][]registerKey, error) {
	ds, err := pebbleds.NewDatastore(executionDataDir, nil)
	if err != nil {
		return nil, fmt.Errorf("could not open execution data datastore %s: %w", executionDataDir, err)
	}
	defer func() {
		if closeErr := ds.Close(); closeErr != nil {
			log.Error().Err(closeErr).Msg("could not close execution data datastore")
		}
	}()

	blobstore := blobs.NewBlobstore(ds)
	executionDataStore := execution_data.NewExecutionDataStore(blobstore, execution_data.DefaultSerializer)

	results, err := ds.Query(ctx, query.Query{})
	if err != nil {
		return nil, fmt.Errorf("could not list execution data blobs of %s: %w", executionDataDir, err)
	}
	defer func() {
		if closeErr := results.Close(); closeErr != nil {
			log.Error().Err(closeErr).Msg("could not close execution data blob listing")
		}
	}()

	blocks := make([][]registerKey, 0, blockCount)
	for len(blocks) < blockCount {
		result, ok := results.NextSync()
		if !ok {
			break
		}

		// the blobstore stores its blobs under the base32 encoding of their multihash
		// the blobstore stores its blobs under a namespace prefix followed by the base32 encoding
		// of their multihash
		dsKey := result.Key
		if index := strings.LastIndex(dsKey, "/"); index > 0 {
			// drop the blobstore's namespace prefix, keeping the leading "/" of the multihash key
			dsKey = dsKey[index:]
		}
		multihash, err := dshelp.DsKeyToMultihash(datastore.RawKey(dsKey))
		if err != nil {
			continue // not a blob of the blobstore
		}

		size, err := blobstore.GetSize(ctx, cid.NewCidV1(cid.Raw, multihash))
		if err != nil {
			continue
		}
		if size > maxRootBlobSize {
			continue // a chunk blob, not the root blob of a block's execution data
		}

		// the execution data store reads a block's execution data by the block's execution data
		// ID, which is the digest of the root blob's multihash
		decoded, err := mh.Decode(multihash)
		if err != nil || decoded.Code != mh.SHA2_256 || len(decoded.Digest) != flow.IdentifierLen {
			continue
		}

		executionData, err := executionDataStore.Get(ctx, flow.HashToID(decoded.Digest))
		if err != nil {
			continue // not the root blob of a block's execution data
		}

		keys := keysOfExecutionData(executionData)
		if len(keys) == 0 {
			continue
		}
		blocks = append(blocks, keys)
	}

	if len(blocks) < blockCount {
		log.Warn().Msgf("found the execution data of %d blocks, which is less than the %d blocks to replay",
			len(blocks), blockCount)
	}
	if len(blocks) == 0 {
		return nil, fmt.Errorf("found the execution data of no block in %s", executionDataDir)
	}

	return blocks, nil
}

// keysOfExecutionData returns the registers the given block touched: the registers of the trie
// update of each of its chunks.
func keysOfExecutionData(executionData *execution_data.BlockExecutionData) []registerKey {
	var keys []registerKey
	for _, chunk := range executionData.ChunkExecutionDatas {
		if chunk.TrieUpdate == nil {
			continue
		}

		for i, path := range chunk.TrieUpdate.Paths {
			key, err := chunk.TrieUpdate.Payloads[i].Key()
			if err != nil {
				continue
			}
			registerID, err := convert.LedgerKeyToRegisterID(key)
			if err != nil {
				continue
			}
			keys = append(keys, registerKey{id: registerID, path: path})
		}
	}
	return keys
}

// flattenBlocks concatenates the registers of the given blocks into one stream of reads.
func flattenBlocks(blocks [][]registerKey) []registerKey {
	reads := 0
	for _, block := range blocks {
		reads += len(block)
	}

	keys := make([]registerKey, 0, reads)
	for _, block := range blocks {
		keys = append(keys, block...)
	}
	return keys
}
