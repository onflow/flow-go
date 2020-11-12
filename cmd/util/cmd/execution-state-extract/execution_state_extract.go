package extract

import (
	"fmt"
	"path"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
)

func getStateCommitment(commits storage.Commits, blockHash flow.Identifier) (flow.StateCommitment, error) {
	return commits.ByBlockID(blockHash)
}

func extractExecutionState(dir string, targetHash flow.StateCommitment, outputDir string, log zerolog.Logger) error {

	led, err := complete.NewLedger(dir, 1000, &metrics.NoopCollector{}, log, nil, complete.DefaultPathFinderVersion)

	filePath := path.Join(outputDir, "root.checkpoint")
	newState, err := led.ExportCheckpointAt(targetHash, []ledger.Migration{noOpMigration}, complete.DefaultPathFinderVersion, filePath)
	if err != nil {
		return fmt.Errorf("cannot create WAL: %w", err)
	}
	log.Info().Msg("New state commitment for the exported state is :" + newState.String())
	return nil
}

func noOpMigration(p []ledger.Payload) ([]ledger.Payload, error) {
	return p, nil
}
