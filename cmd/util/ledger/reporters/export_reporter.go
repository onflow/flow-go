package reporters

import (
	"encoding/json"
	"io/ioutil"

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
	chain                    flow.Chain
	getBeforeMigrationSCFunc GetStateCommitmentFunc
}

func NewExportReporter(
	logger zerolog.Logger,
	chain flow.Chain,
	getBeforeMigrationSCFunc GetStateCommitmentFunc,
) *ExportReporter {
	return &ExportReporter{
		logger:                   logger,
		chain:                    chain,
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
	_ = ioutil.WriteFile("export_report.json", file, 0644)
	return nil
}
