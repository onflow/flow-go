package checkpoint_collect_stats

import (
	"bufio"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/montanaflynn/stats"
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
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/debug"
)

var (
	flagCheckpointDir string
	flagOutputDir     string
	flagMemProfile    bool
)

var Cmd = &cobra.Command{
	Use:   "checkpoint-collect-stats",
	Short: "collects stats on tries stored in a checkpoint",
	Run:   run,
}

func init() {
	Cmd.Flags().StringVar(&flagCheckpointDir, "checkpoint-dir", "",
		"Directory to load checkpoint files from")
	_ = Cmd.MarkFlagRequired("checkpoint-dir")

	Cmd.Flags().StringVar(&flagOutputDir, "output-dir", "",
		"Directory to write checkpoint stats to")
	_ = Cmd.MarkFlagRequired("output-dir")

	Cmd.Flags().BoolVar(&flagMemProfile, "mem-profile", false,
		"Enable memory profiling")
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

func run(*cobra.Command, []string) {

	if flagMemProfile {
		defer profile.Start(profile.MemProfile).Stop()
	}

	memAllocBefore := debug.GetHeapAllocsBytes()
	log.Info().Msgf("loading checkpoint(s) from %v", flagCheckpointDir)

	diskWal, err := wal.NewDiskWAL(zerolog.Nop(), nil, &metrics.NoopCollector{}, flagCheckpointDir, complete.DefaultCacheSize, pathfinder.PathByteSize, wal.SegmentSize)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot create WAL")
	}
	led, err := complete.NewLedger(diskWal, complete.DefaultCacheSize, &metrics.NoopCollector{}, log.Logger, 0)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot create ledger from write-a-head logs and checkpoints")
	}
	compactor, err := complete.NewCompactor(led, diskWal, zerolog.Nop(), complete.DefaultCacheSize, math.MaxInt, 1, atomic.NewBool(false), &metrics.NoopCollector{})
	if err != nil {
		log.Fatal().Err(err).Msg("cannot create compactor")
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
			log.Fatal().Err(err).Msg("cannot load a key")
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
			log.Fatal().Err(err).Msg("cannot compute the sum of values")
		}

		min, err := stats.Min(values)
		if err != nil {
			log.Fatal().Err(err).Msg("cannot compute the min of values")
		}

		percentile25, err := stats.Percentile(values, 25)
		if err != nil {
			log.Fatal().Err(err).Msg("cannot compute the 25th percentile of values")
		}

		median, err := stats.Median(values)
		if err != nil {
			log.Fatal().Err(err).Msg("cannot compute the median of values")
		}

		percentile75, err := stats.Percentile(values, 75)
		if err != nil {
			log.Fatal().Err(err).Msg("cannot compute the 75th percentile of values")
		}

		percentile95, err := stats.Percentile(values, 95)
		if err != nil {
			log.Fatal().Err(err).Msg("cannot compute the 95th percentile of values")
		}

		max, err := stats.Max(values)
		if err != nil {
			log.Fatal().Err(err).Msg("cannot compute the max of values")
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
		log.Fatal().Err(err).Msg("failed to collect stats")
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
		log.Fatal().Err(err).Msg("failed to create path")
	}
	defer fi.Close()

	writer := bufio.NewWriter(fi)
	defer writer.Flush()

	encoder := json.NewEncoder(writer)

	err = encoder.Encode(stats)
	if err != nil {
		log.Fatal().Err(err).Msg("could not json encode ledger stats")
	}
}

func getType(key ledger.Key) string {
	k := key.KeyParts[1].Value
	kstr := string(k)

	if atree.LedgerKeyIsSlabKey(kstr) {
		return "slab"
	}

	switch kstr {
	case "storage":
		return "account's cadence storage domain map"
	case "private":
		return "account's cadence private domain map"
	case "public":
		return "account's cadence public domain map"
	case "contract":
		return "account's cadence contract domain map"
	case flow.ContractNamesKey:
		return "contract names"
	case flow.AccountStatusKey:
		return "account status"
	case "uuid":
		return "uuid generator state"
	case "account_address_state":
		return "address generator state"
	}
	// other fvm registers
	if strings.HasPrefix(kstr, "public_key_") {
		return "public key"
	}
	if strings.HasPrefix(kstr, flow.CodeKeyPrefix) {
		return "contract content"
	}
	return "others"
}
