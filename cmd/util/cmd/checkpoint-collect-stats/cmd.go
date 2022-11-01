package checkpoint_collect_stats

import (
	"bufio"
	"encoding/json"
	"math"
	"os"
	"path/filepath"

	"github.com/pkg/profile"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"go.uber.org/atomic"

	"github.com/onflow/atree"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/debug"
)

var (
	flagCheckpoint string
	flagOutputDir  string
)

var Cmd = &cobra.Command{
	Use:   "checkpoint-collect-stats",
	Short: "collects stats on tries stored in a checkpoint",
	Run:   run,
}

func init() {

	Cmd.Flags().StringVar(&flagCheckpoint, "checkpoint", "",
		"checkpoint file to read")
	_ = Cmd.MarkFlagRequired("checkpoint")

	Cmd.Flags().StringVar(&flagOutputDir, "output-dir", "",
		"Directory to write checkpoint stats to")
	_ = Cmd.MarkFlagRequired("output-dir")

}

type PayloadStats struct {
	TotalPayloadSize           uint64 `json:"total_payload_size"`
	TotalPayloadValueSize      uint64 `json:"total_payload_value_size"`
	TotalPayloadEncodedKeySize uint64 `json:"total_payload_encoded_key_byte_size"`
	TotalPayloadSizeOfTypeSlab uint64 `json:"total_payload_size_of_type_slab"`
}

type Stats struct {
	ledgerStats  *complete.LedgerStats
	payloadStats *PayloadStats
}

func run(*cobra.Command, []string) {

	defer profile.Start(profile.MemProfile).Stop()

	log.Info().Msgf("loading checkpoint %v", flagCheckpoint)

	const (
		checkpointDistance = math.MaxInt // A large number to prevent checkpoint creation.
		checkpointsToKeep  = 1
	)

	memAllocBefore := debug.GetHeapAllocsBytes()

	diskWal, err := wal.NewDiskWAL(zerolog.Nop(), nil, &metrics.NoopCollector{}, flagCheckpoint, complete.DefaultCacheSize, pathfinder.PathByteSize, wal.SegmentSize)
	if err != nil {
		log.Fatal().Err(err).Msgf("cannot create WAL: %w", err)
	}
	led, err := complete.NewLedger(diskWal, complete.DefaultCacheSize, &metrics.NoopCollector{}, log.Logger, 0)
	if err != nil {
		log.Fatal().Err(err).Msgf("cannot create ledger from write-a-head logs and checkpoints: %w", err)
	}

	compactor, err := complete.NewCompactor(led, diskWal, zerolog.Nop(), complete.DefaultCacheSize, checkpointDistance, checkpointsToKeep, atomic.NewBool(false))
	if err != nil {
		log.Fatal().Err(err).Msgf("cannot create compactor: %w", err)
	}
	<-compactor.Ready()
	defer func() {
		<-led.Done()
		<-compactor.Done()
	}()

	memAllocAfter := debug.GetHeapAllocsBytes()
	log.Info().Msgf("the checkpoint is loaded, mem usage: %d", memAllocAfter-memAllocBefore)

	var totalPayloadSize, totalPayloadValueSize, totalPayloadSizeSlabOnly uint64
	var value ledger.Value
	var key ledger.Key
	var valueSize int

	ledgerStats, err := led.CollectStats(func(p *ledger.Payload) {
		totalPayloadSize += uint64(p.Size())
		value = p.Value()
		valueSize = value.Size()
		totalPayloadValueSize += uint64(valueSize)
		key, _ = p.Key()
		// slab types
		if atree.LedgerKeyIsSlabKey(string(key.KeyParts[1].Value)) {
			totalPayloadSizeSlabOnly += uint64(valueSize)
			return
		}
	})

	if err != nil {
		log.Fatal().Err(err).Msgf("failed to collect stats: %w", err)
	}

	stats := &Stats{
		ledgerStats: ledgerStats,
		payloadStats: &PayloadStats{
			TotalPayloadSize:           totalPayloadSize,
			TotalPayloadValueSize:      totalPayloadValueSize,
			TotalPayloadEncodedKeySize: totalPayloadSize - totalPayloadValueSize,
			TotalPayloadSizeOfTypeSlab: totalPayloadSizeSlabOnly,
		},
	}

	path := filepath.Join(flagOutputDir, "ledger.stats.json")

	fi, err := os.Create(path)
	if err != nil {
		log.Fatal().Err(err).Msgf("failed to create path: %w", err)
	}
	defer fi.Close()

	writer := bufio.NewWriter(fi)
	defer writer.Flush()

	jsonData, err := json.Marshal(stats)
	if err != nil {
		log.Fatal().Err(err).Msgf("could not create a json obj for ledger stats: %w", err)
	}
	_, err = writer.WriteString(string(jsonData) + "\n")
	if err != nil {
		log.Fatal().Err(err).Msgf("could not write result json to the file: %w", err)
	}
	writer.Flush()
}
