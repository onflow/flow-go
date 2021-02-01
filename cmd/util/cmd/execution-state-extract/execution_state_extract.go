package extract

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
)

func getStateCommitment(commits storage.Commits, blockHash flow.Identifier) (flow.StateCommitment, error) {
	return commits.ByBlockID(blockHash)
}

func extractExecutionState(dir string, targetHash flow.StateCommitment, outputDir string, log zerolog.Logger) error {

	led, err := complete.NewLedger(
		dir,
		complete.DefaultCacheSize,
		&metrics.NoopCollector{},
		log,
		nil,
		complete.DefaultPathFinderVersion)
	if err != nil {
		return fmt.Errorf("cannot create ledger from write-a-head logs and checkpoints: %w", err)
	}

	newState, err := led.ExportCheckpointAt(targetHash,
		[]ledger.Migration{},
		[]ledger.Reporter{migrations.StorageReporter{Log: log, OutputDir: outputDir}},
		complete.DefaultPathFinderVersion,
		outputDir,
		wal.RootCheckpointFilename)
	if err != nil {
		return fmt.Errorf("cannot generate the output checkpoint: %w", err)
	}

	log.Info().Msgf(
		"New state commitment for the exported state is: %s (base64: %s)",
		newState.String(),
		newState.Base64(),
	)

	return nil
}
