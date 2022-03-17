package reporters

import (
	"encoding/json"
	"io/ioutil"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

type ExportReport struct {
	CurrentStateCommitment  flow.StateCommitment
	EpochCounter            uint64
	PreviousStateCommitment flow.StateCommitment
}

// ExportReporter writes data that can be leveraged outside of extraction
type ExportReporter struct {
	Chain                   flow.Chain
	CurrentStateCommitement flow.StateCommitment
	Log                     zerolog.Logger
	PreviousStateCommitment flow.StateCommitment
}

func (e *ExportReporter) Name() string {
	return "ExportReporter"
}

func (e *ExportReporter) Report(payload []ledger.Payload) error {
	script, _, err := ExecuteCurrentEpochScript(e.Chain, payload)
	if err != nil {
		e.Log.
			Error().
			Err(script.Err).
			Msg("Failed to get epoch counter")
	}
	report := ExportReport{
		CurrentStateCommitment:  e.CurrentStateCommitement,
		EpochCounter:            script.Value.ToGoValue().(uint64),
		PreviousStateCommitment: e.PreviousStateCommitment,
	}
	file, _ := json.MarshalIndent(report, "", " ")
	e.Log.
		Info().
		Str("ExportReport", string(file)).
		Msg("Ledger Export has finished")
	_ = ioutil.WriteFile("export_report.json", file, 0644)
	return nil
}

var _ ledger.Reporter = &ExportReporter{}
