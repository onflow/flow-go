package checkpoint_collect_stats

import (
	"bufio"
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"

	"github.com/montanaflynn/stats"
	"github.com/pkg/profile"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"go.uber.org/atomic"

	"github.com/onflow/atree"
	"github.com/onflow/flow-go/fvm/state"
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

type Stats struct {
	LedgerStats  *complete.LedgerStats
	PayloadStats *PayloadStats
}

type PayloadStats struct {
	TotalPayloadSize      uint64                 `json:"total_payload_size"`
	TotalPayloadValueSize uint64                 `json:"total_payload_value_size"`
	StatsByTypes          []RegisterStatsByTypes `json:"stats_by_types"`
}

type RegisterStatsByTypes struct {
	Type                    string  `json:"type"`
	Counts                  uint64  `json:"counts"`
	ValueSizeTotal          float64 `json:"value_size_total"`
	ValueSizeMin            float64 `json:"value_size_min"`
	ValueSize25thPercentile float64 `json:"value_size_25th_percentile"`
	ValueSizeMedian         float64 `json:"value_size_median"`
	ValueSize75thPercentile float64 `json:"value_size_75th_percentile"`
	ValueSize95thPercentile float64 `json:"value_size_95th_percentile"`
	ValueSizeMax            float64 `json:"value_size_max"`
}

type sizesByType map[string][]float64

func getType(key ledger.Key) string {
	k := key.KeyParts[1].Value
	kstr := string(k)
	if atree.LedgerKeyIsSlabKey(kstr) {
		return "slab"
	}
	if bytes.HasPrefix(k, []byte("public_key_")) {
		return "public key"
	}
	if kstr == state.KeyContractNames {
		return "contract names"
	}
	if bytes.HasPrefix(k, []byte(state.KeyCode)) {
		return "contract content"
	}
	if kstr == state.KeyAccountStatus {
		return "account status"
	}
	return "others"
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

	var totalPayloadSize, totalPayloadValueSize uint64
	var value ledger.Value
	var key ledger.Key
	var size, valueSize int

	valueSizesByType := make(sizesByType, 0)
	ledgerStats, err := led.CollectStats(func(p *ledger.Payload) {

		key, err = p.Key()
		if err != nil {
			log.Fatal().Err(err).Msgf("cannot loading the key: %w", err)
		}

		size = p.Size()
		value = p.Value()
		valueSize = value.Size()
		totalPayloadSize += uint64(size)
		totalPayloadValueSize += uint64(valueSize)
		valueSizesByType[getType(key)] = append(valueSizesByType[getType(key)], float64(valueSize))
	})

	statsByTypes := make([]RegisterStatsByTypes, 0)
	for t, values := range valueSizesByType {

		sum, err := stats.Sum(values)
		if err != nil {
			log.Fatal().Err(err).Msgf("cannot compute the sum of values: %w", err)
		}

		min, err := stats.Min(values)
		if err != nil {
			log.Fatal().Err(err).Msgf("cannot compute the min of values: %w", err)
		}

		percentile25, err := stats.Percentile(values, 0.25)
		if err != nil {
			log.Fatal().Err(err).Msgf("cannot compute the 25th percentile of values: %w", err)
		}

		median, err := stats.Median(values)
		if err != nil {
			log.Fatal().Err(err).Msgf("cannot compute the median of values: %w", err)
		}

		percentile75, err := stats.Percentile(values, 0.75)
		if err != nil {
			log.Fatal().Err(err).Msgf("cannot compute the 75th percentile of values: %w", err)
		}

		percentile95, err := stats.Percentile(values, 0.95)
		if err != nil {
			log.Fatal().Err(err).Msgf("cannot compute the 95th percentile of values: %w", err)
		}

		max, err := stats.Max(values)
		if err != nil {
			log.Fatal().Err(err).Msgf("cannot compute the max of values: %w", err)
		}

		statsByTypes = append(statsByTypes,
			RegisterStatsByTypes{
				Type:                    t,
				Counts:                  uint64(len(values)),
				ValueSizeTotal:          sum,
				ValueSizeMin:            min,
				ValueSize25thPercentile: percentile25,
				ValueSizeMedian:         median,
				ValueSize75thPercentile: percentile75,
				ValueSize95thPercentile: percentile95,
				ValueSizeMax:            max,
			})
	}

	if err != nil {
		log.Fatal().Err(err).Msgf("failed to collect stats: %w", err)
	}

	stats := &Stats{
		LedgerStats: ledgerStats,
		PayloadStats: &PayloadStats{
			TotalPayloadSize:      totalPayloadSize,
			TotalPayloadValueSize: totalPayloadValueSize,
			StatsByTypes:          statsByTypes,
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
