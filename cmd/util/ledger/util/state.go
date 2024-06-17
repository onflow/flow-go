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
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
)

func ReadTrie(dir string, targetHash flow.StateCommitment) ([]*ledger.Payload, error) {
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
		s, _ := led.MostRecentTouchedState()
		log.Info().
			Str("hash", s.String()).
			Msgf("Most recently touched state")
		return nil, fmt.Errorf("cannot get trie at the given state commitment: %w", err)
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
