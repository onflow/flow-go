package util

import (
	"encoding/hex"
	"fmt"
	"math"

	"github.com/rs/zerolog/log"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete"
	mtrie "github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/ledger/complete/payloadless"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
)

func ReadTrie(dir string, targetHash flow.StateCommitment) (*mtrie.MTrie, error) {
	log.Info().Msg("init WAL")

	diskWal, err := wal.NewDiskWAL(
		log.Logger,
		nil,
		metrics.NewNoopCollector(),
		dir,
		complete.DefaultCacheSize,
		pathfinder.PathByteSize,
		wal.SegmentSize,
	)
	if err != nil {
		return nil, fmt.Errorf("cannot create disk WAL: %w", err)
	}

	log.Info().Msg("init ledger")

	led, err := complete.NewLedger(
		diskWal,
		complete.DefaultCacheSize,
		&metrics.NoopCollector{},
		log.Logger,
		complete.DefaultPathFinderVersion)
	if err != nil {
		return nil, fmt.Errorf("cannot create ledger from write-a-head logs and checkpoints: %w", err)
	}

	const (
		checkpointDistance = math.MaxInt // A large number to prevent checkpoint creation.
		checkpointsToKeep  = 1
	)

	log.Info().Msg("init compactor")

	compactor, err := complete.NewCompactor(
		led,
		diskWal,
		log.Logger,
		complete.DefaultCacheSize,
		checkpointDistance,
		checkpointsToKeep,
		atomic.NewBool(false),
		&metrics.NoopCollector{},
	)
	if err != nil {
		return nil, fmt.Errorf("cannot create compactor: %w", err)
	}

	log.Info().Msgf("waiting for compactor to load checkpoint and WAL")

	<-compactor.Ready()

	defer func() {
		<-led.Done()
		<-compactor.Done()
	}()

	state := ledger.State(targetHash)

	trie, err := led.Trie(ledger.RootHash(state))
	if err != nil {
		s, err2 := led.MostRecentTouchedState()
		if err2 != nil {
			log.Error().Err(err2).
				Msgf("cannot get most recently touched state in %v, check the --execution-state-dir flag", dir)
		} else if s == ledger.State(mtrie.NewEmptyMTrie().RootHash()) {
			log.Error().Msgf("cannot find any trie in folder %v. check the --execution-state-dir flag", dir)
		} else {
			log.Info().
				Str("hash", s.String()).
				Msgf("Most recently touched state")
		}
		return nil, fmt.Errorf("cannot get trie at the given state commitment: %w", err)
	}

	return trie, nil
}

// ReadPayloadlessTrie loads the payloadless (V7) trie at the given state commitment from the WAL
// directory, recovering in-memory state from the latest V7 checkpoint plus any newer WAL segments.
// It is the payloadless counterpart of [ReadTrie]: the returned trie's leaves carry a 32-byte leaf
// hash, not a full payload.
//
// `capacity` bounds the number of tries retained in the forest during replay (the peak-memory
// driver). It should match the node's `--mtrie-cache-size` ([ledger.DefaultMTrieCacheSize]) so this
// tool's memory footprint matches a node booting at the same state; a smaller value trades safety
// against WAL forks for lower memory.
//
// WAL replay stops as soon as the target trie is produced (see
// [wal.DiskWAL.ReplayOnPayloadlessForestUntil]), so it does not read segments past the target.
// This is what lets an older state commitment be extracted at all: replaying to the WAL tip would
// evict the target from the LRU-bounded forest before it could be read.
//
// This is a read-only load: no checkpoint is written and no compactor is started. The exclusive WAL
// directory lock acquired on open is released before this returns, and only the returned trie's
// reachable nodes stay resident for any downstream checkpoint writing.
//
// No error returns are expected during normal operation.
func ReadPayloadlessTrie(dir string, targetHash flow.StateCommitment, capacity int) (*payloadless.MTrie, error) {
	log.Info().Msg("init WAL")

	diskWal, err := wal.NewDiskWAL(
		log.Logger,
		nil,
		metrics.NewNoopCollector(),
		dir,
		capacity,
		pathfinder.PathByteSize,
		wal.SegmentSize,
	)
	if err != nil {
		return nil, fmt.Errorf("cannot create disk WAL: %w", err)
	}

	// Done closes the WAL and releases the exclusive directory lock.
	defer func() {
		<-diskWal.Done()
	}()

	forest, err := payloadless.NewForest(capacity, metrics.NewNoopCollector(), nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create payloadless forest: %w", err)
	}

	targetRootHash := ledger.RootHash(targetHash)

	log.Info().Msg("loading V7 checkpoint and replaying WAL until the target trie is found")

	found, err := diskWal.ReplayOnPayloadlessForestUntil(forest, targetRootHash)
	if err != nil {
		return nil, fmt.Errorf("cannot replay payloadless WAL: %w", err)
	}
	if !found {
		return nil, fmt.Errorf(
			"no payloadless trie with state commitment %x was found in %s; check the --state-commitment and --execution-state-dir flags",
			targetHash[:], dir)
	}

	trie, err := forest.GetTrie(targetRootHash)
	if err != nil {
		return nil, fmt.Errorf("cannot get payloadless trie at state commitment %x: %w", targetHash[:], err)
	}

	return trie, nil
}

func ReadTrieForPayloads(dir string, targetHash flow.StateCommitment) ([]*ledger.Payload, error) {
	trie, err := ReadTrie(dir, targetHash)
	if err != nil {
		return nil, err
	}
	return trie.AllPayloads(), nil
}

func ParseStateCommitment(stateCommitmentHex string) flow.StateCommitment {
	var err error
	stateCommitmentBytes, err := hex.DecodeString(stateCommitmentHex)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot get decode the state commitment")
	}

	stateCommitment, err := flow.ToStateCommitment(stateCommitmentBytes)
	if err != nil {
		log.Fatal().Err(err).Msg("invalid state commitment length")
	}

	return stateCommitment
}
