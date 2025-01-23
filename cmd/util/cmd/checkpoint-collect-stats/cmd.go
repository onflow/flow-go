package checkpoint_collect_stats

import (
	"cmp"
	"encoding/hex"
	"math"
	"slices"
	"strings"
	"sync"

	"github.com/montanaflynn/stats"
	"github.com/pkg/profile"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"go.uber.org/atomic"

	"github.com/onflow/atree"

	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/fvm/evm/emulator/state"
	"github.com/onflow/flow-go/fvm/evm/handler"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/debug"
)

var (
	flagCheckpointDir   string
	flagStateCommitment string
	flagPayloads        string
	flagOutputDir       string
	flagChain           string
	flagTopN            int
	flagMemProfile      bool
)

const (
	ledgerStatsReportName  = "ledger-stats"
	accountStatsReportName = "account-stats"
)

const (
	domainTypePrefix         = "domain "
	payloadChannelBufferSize = 100_000
	initialAccountMapSize    = 5_000_000
)

const (
	// EVM register keys from fvm/evm/handler/blockHashList.go
	blockHashListMetaKey         = "BlockHashListMeta"
	blockHashListBucketKeyPrefix = "BlockHashListBucket"
)

var Cmd = &cobra.Command{
	Use:   "checkpoint-collect-stats",
	Short: "collects stats on tries stored in a checkpoint, or payloads from a payloads file",
	Long: `checkpoint-collect-stats collects stats on tries stored in a checkpoint, or payloads from a payloads file.
Two kinds of input data are supported:
- checkpoint file(s) ("--checkpoint-dir" with optional "--state-commitment"), or
- payloads file ("--payload-filename")`,
	Run: run,
}

func init() {
	Cmd.Flags().StringVar(&flagCheckpointDir, "checkpoint-dir", "",
		"Directory to load checkpoint files from")

	// state-commitment is optional.
	// When provided, this program only gathers stats on trie with matching state commitment.
	Cmd.Flags().StringVar(&flagStateCommitment, "state-commitment", "",
		"Trie state commitment")

	Cmd.Flags().StringVar(&flagPayloads, "payload-filename", "",
		"Payloads file name to load payloads from")

	Cmd.Flags().StringVar(&flagOutputDir, "output-dir", "",
		"Directory to write checkpoint stats to")
	_ = Cmd.MarkFlagRequired("output-dir")

	Cmd.Flags().IntVar(&flagTopN, "top-n", 10,
		"number of largest payloads or accounts to report")

	Cmd.Flags().StringVar(&flagChain, "chain", "", "Chain name")
	_ = Cmd.MarkFlagRequired("chain")

	Cmd.Flags().BoolVar(&flagMemProfile, "mem-profile", false,
		"Enable memory profiling")
}

type Stats struct {
	LedgerStats  *complete.LedgerStats `json:",omitempty"`
	PayloadStats *PayloadStats
}

type PayloadStats struct {
	TotalPayloadCount     uint64                 `json:"total_payload_count"`
	TotalPayloadSize      uint64                 `json:"total_payload_size"`
	TotalPayloadValueSize uint64                 `json:"total_payload_value_size"`
	StatsByTypes          []RegisterStatsByTypes `json:"stats_by_types"`
	TopN                  []PayloadInfo          `json:"largest_payloads"`
}

type RegisterStatsByTypes struct {
	Type                    string                 `json:"type"`
	Counts                  uint64                 `json:"counts"`
	ValueSizeTotal          float64                `json:"value_size_total"`
	ValueSizeMin            float64                `json:"value_size_min"`
	ValueSize25thPercentile float64                `json:"value_size_25th_percentile"`
	ValueSizeMedian         float64                `json:"value_size_median"`
	ValueSize75thPercentile float64                `json:"value_size_75th_percentile"`
	ValueSize95thPercentile float64                `json:"value_size_95th_percentile"`
	ValueSize99thPercentile float64                `json:"value_size_99th_percentile"`
	ValueSizeMax            float64                `json:"value_size_max"`
	SubTypes                []RegisterStatsByTypes `json:"subtypes,omitempty"`
}

type PayloadInfo struct {
	Address string `json:"address"`
	Key     string `json:"key"`
	Type    string `json:"type"`
	Size    uint64 `json:"size"`
}

type AccountStats struct {
	AccountCount              uint64         `json:"total_account_count"`
	AccountSizeMin            float64        `json:"account_size_min"`
	AccountSize25thPercentile float64        `json:"account_size_25th_percentile"`
	AccountSizeMedian         float64        `json:"account_size_median"`
	AccountSize75thPercentile float64        `json:"account_size_75th_percentile"`
	AccountSize95thPercentile float64        `json:"account_size_95th_percentile"`
	AccountSize99thPercentile float64        `json:"account_size_99th_percentile"`
	AccountSizeMax            float64        `json:"account_size_max"`
	ServiceAccount            *AccountInfo   `json:"service_account,omitempty"`
	EVMAccount                *AccountInfo   `json:"evm_account,omitempty"`
	TopN                      []*AccountInfo `json:"largest_accounts"`
}

type AccountInfo struct {
	Address      string `json:"address"`
	PayloadCount uint64 `json:"payload_count"`
	PayloadSize  uint64 `json:"payload_size"`
}

type sizesByType map[string][]float64

func run(*cobra.Command, []string) {

	if flagPayloads == "" && flagCheckpointDir == "" {
		log.Fatal().Msg("Either --payload-filename or --checkpoint-dir must be provided")
	}
	if flagPayloads != "" && flagCheckpointDir != "" {
		log.Fatal().Msg("Only one of --payload-filename or --checkpoint-dir must be provided")
	}
	if flagCheckpointDir == "" && flagStateCommitment != "" {
		log.Fatal().Msg("--checkpont-dir must be provided when --state-commitment is provided")
	}

	chainID := flow.ChainID(flagChain)
	// Validate chain ID
	_ = chainID.Chain()

	if flagMemProfile {
		defer profile.Start(profile.MemProfile).Stop()
	}

	payloadChannel := make(chan *ledger.Payload, payloadChannelBufferSize)

	ledgerStatsChannel := make(chan *complete.LedgerStats, 1)

	// Load execution state and retrieve payloads async
	go getPayloadsAsync(payloadChannel, ledgerStatsChannel)

	var totalPayloadCount, totalPayloadSize, totalPayloadValueSize uint64

	largestPayloads := util.NewTopN[PayloadInfo](
		flagTopN,
		func(a, b PayloadInfo) bool {
			return a.Size < b.Size
		},
	)

	valueSizesByType := make(sizesByType, 0)

	accounts := make(map[string]*AccountInfo, initialAccountMapSize)

	// Process payloads until payloadChannel is closed
	for p := range payloadChannel {
		key, err := p.Key()
		if err != nil {
			log.Fatal().Err(err).Msg("cannot load a key")
		}

		address := key.KeyParts[0].Value

		size := p.Size()
		value := p.Value()
		valueSize := value.Size()

		// Update total payload size and count
		totalPayloadSize += uint64(size)
		totalPayloadValueSize += uint64(valueSize)
		totalPayloadCount++

		// Update payload sizes by type
		typ := getType(key)
		valueSizesByType[typ] = append(valueSizesByType[typ], float64(valueSize))

		// Update top N largest payloads
		_, _ = largestPayloads.Add(
			PayloadInfo{
				Address: hex.EncodeToString(address),
				Key:     hex.EncodeToString(key.KeyParts[1].Value),
				Type:    typ,
				Size:    uint64(valueSize),
			})

		// Update accounts
		account, exist := accounts[string(address)]
		if !exist {
			account = &AccountInfo{
				Address: hex.EncodeToString(address),
			}
			accounts[string(address)] = account
		}
		account.PayloadCount++
		account.PayloadSize += uint64(size)
	}

	// At this point, all payload are processed.

	ledgerStats := <-ledgerStatsChannel

	var wg sync.WaitGroup
	wg.Add(2)

	// Collect and write ledger stats
	go func() {
		defer wg.Done()

		statsByTypes := getStats(valueSizesByType)

		// Sort top N largest payloads by payload size in descending order
		slices.SortFunc(largestPayloads.Tree, func(a, b PayloadInfo) int {
			return cmp.Compare(b.Size, a.Size)
		})

		stats := &Stats{
			LedgerStats: ledgerStats,
			PayloadStats: &PayloadStats{
				TotalPayloadCount:     totalPayloadCount,
				TotalPayloadSize:      totalPayloadSize,
				TotalPayloadValueSize: totalPayloadValueSize,
				StatsByTypes:          statsByTypes,
				TopN:                  largestPayloads.Tree,
			},
		}

		writeStats(ledgerStatsReportName, stats)
	}()

	// Collect and write account stats
	go func() {
		defer wg.Done()

		accountsSlice := make([]*AccountInfo, 0, len(accounts))
		accountSizesSlice := make([]float64, 0, len(accounts))

		for _, acct := range accounts {
			accountsSlice = append(accountsSlice, acct)
			accountSizesSlice = append(accountSizesSlice, float64(acct.PayloadSize))
		}

		// Sort accounts by payload size in descending order
		slices.SortFunc(accountsSlice, func(a, b *AccountInfo) int {
			return cmp.Compare(b.PayloadSize, a.PayloadSize)
		})

		stats := getTypeStats("", accountSizesSlice)

		evmAccountAddress := systemcontracts.SystemContractsForChain(chainID).EVMStorage.Address

		serviceAccountAddress := serviceAccountAddressForChain(chainID)

		acctStats := &AccountStats{
			AccountCount:              uint64(len(accountsSlice)),
			ServiceAccount:            accounts[string(serviceAccountAddress[:])],
			EVMAccount:                accounts[string(evmAccountAddress[:])],
			TopN:                      accountsSlice[:flagTopN],
			AccountSizeMin:            stats.ValueSizeMin,
			AccountSize25thPercentile: stats.ValueSize25thPercentile,
			AccountSizeMedian:         stats.ValueSizeMedian,
			AccountSize75thPercentile: stats.ValueSize75thPercentile,
			AccountSize95thPercentile: stats.ValueSize95thPercentile,
			AccountSize99thPercentile: stats.ValueSize99thPercentile,
			AccountSizeMax:            stats.ValueSizeMax,
		}

		writeStats(accountStatsReportName, acctStats)
	}()

	wg.Wait()
}

func getPayloadsAsync(
	payloadChannel chan<- *ledger.Payload,
	ledgerStatsChannel chan<- *complete.LedgerStats,
) {
	defer close(payloadChannel)
	defer close(ledgerStatsChannel)

	payloadCallback := func(payload *ledger.Payload) {
		payloadChannel <- payload
	}

	useCheckpointFile := flagPayloads == ""

	if useCheckpointFile {
		ledgerStatsChannel <- getPayloadStatsFromCheckpoint(payloadCallback)
	} else {
		getPayloadStatsFromPayloadFile(payloadCallback)
	}
}

func getPayloadStatsFromPayloadFile(payloadCallBack func(payload *ledger.Payload)) {
	memAllocBefore := debug.GetHeapAllocsBytes()
	log.Info().Msgf("loading payloads from %v", flagPayloads)

	_, payloads, err := util.ReadPayloadFile(log.Logger, flagPayloads)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to read payloads")
	}

	memAllocAfter := debug.GetHeapAllocsBytes()
	log.Info().Msgf("%d payloads are loaded, mem usage: %d", len(payloads), memAllocAfter-memAllocBefore)

	for _, p := range payloads {
		payloadCallBack(p)
	}
}

func getPayloadStatsFromCheckpoint(payloadCallBack func(payload *ledger.Payload)) *complete.LedgerStats {
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

	var tries []*trie.MTrie

	if flagStateCommitment != "" {
		stateCommitment := util.ParseStateCommitment(flagStateCommitment)

		t, err := led.FindTrieByStateCommit(stateCommitment)
		if err != nil {
			log.Fatal().Err(err).Msgf("failed to find trie with state commitment %x", stateCommitment)
		}
		if t == nil {
			log.Fatal().Msgf("no trie with state commitment %x", stateCommitment)
		}

		tries = append(tries, t)
	} else {
		ts, err := led.Tries()
		if err != nil {
			log.Fatal().Err(err).Msg("failed to get tries")
		}

		tries = append(tries, ts...)
	}

	log.Info().Msgf("collecting stats on %d tries", len(tries))

	ledgerStats, err := complete.CollectStats(tries, payloadCallBack)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to collect stats")
	}

	return ledgerStats
}

func getTypeStats(t string, values []float64) RegisterStatsByTypes {
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

	percentile99, err := stats.Percentile(values, 99)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot compute the 99th percentile of values")
	}

	max, err := stats.Max(values)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot compute the max of values")
	}

	return RegisterStatsByTypes{
		Type:                    t,
		Counts:                  uint64(len(values)),
		ValueSizeTotal:          sum,
		ValueSizeMin:            min,
		ValueSize25thPercentile: percentile25,
		ValueSizeMedian:         median,
		ValueSize75thPercentile: percentile75,
		ValueSize95thPercentile: percentile95,
		ValueSize99thPercentile: percentile99,
		ValueSizeMax:            max,
	}
}

func getStats(valueSizesByType sizesByType) []RegisterStatsByTypes {
	domainStats := make([]RegisterStatsByTypes, 0, len(util.StorageMapDomains))
	var allDomainSizes []float64

	statsByTypes := make([]RegisterStatsByTypes, 0, len(valueSizesByType))
	for t, values := range valueSizesByType {

		stats := getTypeStats(t, values)

		if isDomainType(t) {
			domainStats = append(domainStats, stats)
			allDomainSizes = append(allDomainSizes, values...)
		} else {
			statsByTypes = append(statsByTypes, stats)
		}
	}

	allDomainStats := getTypeStats("domain", allDomainSizes)
	allDomainStats.SubTypes = domainStats

	statsByTypes = append(statsByTypes, allDomainStats)

	// Sort domain stats by payload count in descending order
	slices.SortFunc(allDomainStats.SubTypes, func(a, b RegisterStatsByTypes) int {
		return cmp.Compare(b.Counts, a.Counts)
	})

	// Sort stats by payload count in descending order
	slices.SortFunc(statsByTypes, func(a, b RegisterStatsByTypes) int {
		return cmp.Compare(b.Counts, a.Counts)
	})

	return statsByTypes
}

func writeStats(reportName string, stats interface{}) {
	rw := reporters.NewReportFileWriterFactory(flagOutputDir, log.Logger).
		ReportWriter(reportName)
	defer rw.Close()

	rw.Write(stats)
}

func isDomainType(typ string) bool {
	return strings.HasPrefix(typ, domainTypePrefix)
}

func getType(key ledger.Key) string {
	k := key.KeyParts[1].Value
	kstr := string(k)

	if atree.LedgerKeyIsSlabKey(kstr) {
		return "atree slab"
	}

	isDomain := slices.Contains(util.StorageMapDomains, kstr)
	if isDomain {
		return domainTypePrefix + kstr
	}

	switch kstr {
	case flow.ContractNamesKey:
		return "contract names"
	case flow.AccountStatusKey:
		return "account status"
	case flow.AddressStateKey:
		return "address generator state"
	case state.AccountsStorageIDKey:
		return "account storage ID"
	case state.CodesStorageIDKey:
		return "code storage ID"
	case handler.BlockStoreLatestBlockKey:
		return "latest block"
	case handler.BlockStoreLatestBlockProposalKey:
		return "latest block proposal"
	}

	// other fvm registers
	if kstr == "uuid" || strings.HasPrefix(kstr, "uuid_") {
		return "uuid generator state"
	}
	if strings.HasPrefix(kstr, "public_key_") {
		return "public key"
	}
	if strings.HasPrefix(kstr, flow.CodeKeyPrefix) {
		return "contract content"
	}

	// other evm registers
	if strings.HasPrefix(kstr, blockHashListBucketKeyPrefix) {
		return "block hash list bucket"
	}
	if strings.HasPrefix(kstr, blockHashListMetaKey) {
		return "block hash list meta"
	}

	log.Warn().Msgf("unknown payload key: %s", kstr)

	return "others"
}

func serviceAccountAddressForChain(chainID flow.ChainID) flow.Address {
	sc := systemcontracts.SystemContractsForChain(chainID)
	return sc.FlowServiceAccount.Address
}
