package reporters

import (
	"encoding/json"
	"os"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

var _ ledger.Reporter = &ExportReporter{}

type GetStateCommitmentFunc func() flow.StateCommitment

type ExportReport struct {
	EpochCounter            uint64
	PreviousStateCommitment flow.StateCommitment
	CurrentStateCommitment  flow.StateCommitment
	ReportSucceeded         bool
}

// ExportReporter writes data that can be leveraged outside of extraction
type ExportReporter struct {
	logger                   zerolog.Logger
	getBeforeMigrationSCFunc GetStateCommitmentFunc
}

func NewExportReporter(
	logger zerolog.Logger,
	getBeforeMigrationSCFunc GetStateCommitmentFunc,
) *ExportReporter {
	return &ExportReporter{
		logger:                   logger,
		getBeforeMigrationSCFunc: getBeforeMigrationSCFunc,
	}
}

func (e *ExportReporter) Name() string {
	return "ExportReporter"
}

func (e *ExportReporter) Report(payload []ledger.Payload, commit ledger.State) error {
	report := ExportReport{
		EpochCounter:            0, // it will be overwritten by external automation, and this field will be removed later.
		PreviousStateCommitment: e.getBeforeMigrationSCFunc(),
		CurrentStateCommitment:  flow.StateCommitment(commit),
		ReportSucceeded:         true,
	}
	file, _ := json.MarshalIndent(report, "", " ")
	e.logger.
		Info().
		Str("ExportReport", string(file)).
		Msg("Ledger Export has finished")
	_ = os.WriteFile("export_report.json", file, 0644)
	return nil
}
