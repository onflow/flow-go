package migrate

import (
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/ledger/storage"
	"github.com/onflow/flow-go/module/metrics"
)

var (
	flagCheckpointDir string
)

var Cmd = &cobra.Command{
	Use:   "migrate-checkpoint-payload",
	Short: "read the checkpoint file and store all payloads to storage",
	Run:   run,
}

func init() {
	Cmd.Flags().StringVar(&flagCheckpointDir, "checkpoint-dir", "",
		"Directory to load checkpoint files from")
	_ = Cmd.MarkFlagRequired("checkpoint-dir")
}

func run(*cobra.Command, []string) {
	diskWal, err := wal.NewDiskWAL(zerolog.Nop(), nil, &metrics.NoopCollector{}, flagCheckpointDir, complete.DefaultCacheSize, pathfinder.PathByteSize, wal.SegmentSize)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot create WAL")
	}
	payloadStorage := storage.CreatePayloadStorage()

}
