// execution-data-payload-stats analyzes execution data by height and counts payload values by type in TrieUpdate records.
//
// Usage:
//
//	go run cmd/util/cmd/execution-data-payload-stats/main.go --blobstore-dir /path/to/execution_data_blobstore --datadir /path/to/protocol --from 100 --to 200
//
// The utility prints three counts:
//  1. Number of payloads with nil values
//  2. Number of payloads with empty slice []byte{} values
//  3. Number of payloads with non-empty values
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	pebbleds "github.com/ipfs/go-ds-pebble"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	execdatacache "github.com/onflow/flow-go/module/executiondatasync/execution_data/cache"
	herocache "github.com/onflow/flow-go/module/mempool/herocache"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
)

func main() {
	var blobstoreDir, datadir string
	var fromHeight, toHeight uint64
	flag.StringVar(&blobstoreDir, "blobstore-dir", "", "Directory containing execution data blobstore")
	flag.StringVar(&datadir, "datadir", "", "Directory containing protocol database")
	flag.Uint64Var(&fromHeight, "from", 0, "Starting block height (inclusive)")
	flag.Uint64Var(&toHeight, "to", 0, "Ending block height (inclusive)")
	flag.Parse()

	if blobstoreDir == "" {
		fmt.Fprintf(os.Stderr, "Error: --blobstore-dir flag is required\n")
		flag.Usage()
		os.Exit(1)
	}
	if datadir == "" {
		fmt.Fprintf(os.Stderr, "Error: --datadir flag is required\n")
		flag.Usage()
		os.Exit(1)
	}

	// Initialize logger with timestamps to stderr
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()

	// Log input
	logger.Info().
		Str("blobstore_dir", blobstoreDir).
		Str("datadir", datadir).
		Uint64("from", fromHeight).
		Uint64("to", toHeight).
		Msg("Starting execution data payload analysis")

	// Validate height range
	if toHeight < fromHeight {
		logger.Fatal().Msg("to height must be >= from height")
	}

	// Initialize execution data blobstore
	ds, err := pebbleds.NewDatastore(blobstoreDir, nil)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize execution data blobstore")
	}
	defer ds.Close()

	blobstore := blobs.NewBlobstore(ds)
	executionDataStore := execution_data.NewExecutionDataStore(blobstore, execution_data.DefaultSerializer)

	// Initialize protocol database
	db, err := common.InitStorage(datadir)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize protocol database")
	}
	defer db.Close()

	// Initialize storages
	storages := common.InitStorages(db)

	// Create a small execution data mempool cache for the utility
	execDataMempoolCache := herocache.NewBlockExecutionData(
		100, // small cache limit for utility
		logger,
		metrics.NewNoopCollector(),
	)

	// Create execution data cache
	execDataCache := execdatacache.NewExecutionDataCache(
		executionDataStore,
		storages.Headers,
		storages.Seals,
		storages.Results,
		execDataMempoolCache,
	)

	// Count payloads by value type
	var nilCount, emptySliceCount, nonEmptyCount int
	var blocksProcessed, chunksProcessed int

	ctx := context.Background()

	// Process each height
	for height := fromHeight; height <= toHeight; height++ {
		logger.Debug().Uint64("height", height).Msg("Processing block height")

		// Get execution data for this height
		execDataEntity, err := execDataCache.ByHeight(ctx, height)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				logger.Warn().Uint64("height", height).Msg("Execution data not found for height, skipping")
				continue
			}
			logger.Fatal().Err(err).Uint64("height", height).Msg("Failed to get execution data")
		}

		blocksProcessed++

		// Process each chunk's TrieUpdate
		for _, chunkData := range execDataEntity.BlockExecutionData.ChunkExecutionDatas {
			if chunkData.TrieUpdate == nil {
				continue
			}

			chunksProcessed++

			// Check all payloads in the TrieUpdate
			for _, payload := range chunkData.TrieUpdate.Payloads {
				if payload == nil {
					continue
				}

				value := payload.Value()
				if value == nil {
					nilCount++
				} else if len(value) == 0 {
					emptySliceCount++
				} else {
					nonEmptyCount++
				}
			}
		}
	}

	logger.Info().
		Uint64("from", fromHeight).
		Uint64("to", toHeight).
		Int("blocks_processed", blocksProcessed).
		Int("chunks_processed", chunksProcessed).
		Msg("Finished processing execution data")

	// Print results
	fmt.Printf("Payload statistics:\n")
	fmt.Printf("  nil values:        %d\n", nilCount)
	fmt.Printf("  empty slice []byte{}: %d\n", emptySliceCount)
	fmt.Printf("  non-empty values:  %d\n", nonEmptyCount)
	fmt.Printf("  total payloads:    %d\n", nilCount+emptySliceCount+nonEmptyCount)
}
