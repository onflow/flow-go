// checkpoint-payload-stats analyzes a root checkpoint file and counts payload values by type.
//
// Usage:
//
//	go run cmd/util/cmd/checkpoint-payload-stats/main.go --checkpoint /path/to/root.checkpoint
//
// The utility prints three counts:
//  1. Number of payloads with nil values
//  2. Number of payloads with empty slice []byte{} values
//  3. Number of payloads with non-empty values
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/ledger/complete/wal"
)

func main() {
	var checkpointPath string
	flag.StringVar(&checkpointPath, "checkpoint", "", "Path to root checkpoint file (e.g., /path/to/root.checkpoint)")
	flag.Parse()

	if checkpointPath == "" {
		fmt.Fprintf(os.Stderr, "Error: --checkpoint flag is required\n")
		flag.Usage()
		os.Exit(1)
	}

	// Initialize logger
	logger := zerolog.Nop()

	// Log input
	log.Info().
		Str("checkpoint_path", checkpointPath).
		Msg("Starting checkpoint payload analysis")

	// Extract directory and filename from the checkpoint path
	dir := filepath.Dir(checkpointPath)
	fileName := filepath.Base(checkpointPath)

	log.Info().
		Str("checkpoint_dir", dir).
		Str("checkpoint_file", fileName).
		Msg("Parsed checkpoint path")

	// Read root hash(es) from checkpoint
	rootHashes, err := wal.ReadTriesRootHash(logger, dir, fileName)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to read root hash from checkpoint")
	}

	if len(rootHashes) == 0 {
		log.Fatal().Msg("No root hash found in checkpoint file")
	}

	if len(rootHashes) > 1 {
		log.Warn().
			Int("root_count", len(rootHashes)).
			Msg("Checkpoint contains multiple root hashes, using the first one")
	}

	rootHash := rootHashes[0]

	// Create channel for leaf nodes
	leafNodeChan := make(chan *wal.LeafNode, 100)

	// Read leaf nodes in a goroutine
	go func() {
		err := wal.OpenAndReadLeafNodesFromCheckpointV6(leafNodeChan, dir, fileName, rootHash, logger)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to read leaf nodes from checkpoint")
		}
	}()

	// Count payloads by value type
	var nilCount, emptySliceCount, nonEmptyCount int

	for leafNode := range leafNodeChan {
		if leafNode.Payload == nil {
			continue
		}

		value := leafNode.Payload.Value()
		if value == nil {
			nilCount++
		} else if len(value) == 0 {
			emptySliceCount++
		} else {
			nonEmptyCount++
		}
	}

	// Print results
	fmt.Printf("Payload statistics:\n")
	fmt.Printf("  nil values:        %d\n", nilCount)
	fmt.Printf("  empty slice []byte{}: %d\n", emptySliceCount)
	fmt.Printf("  non-empty values:  %d\n", nonEmptyCount)
	fmt.Printf("  total payloads:    %d\n", nilCount+emptySliceCount+nonEmptyCount)
}
