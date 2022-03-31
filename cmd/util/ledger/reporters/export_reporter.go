package reporters

import (
	"encoding/json"
	"io/ioutil"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

type ExportReport struct {
	CurrentStateCommitment  string
	EpochCounter            uint64
	PreviousStateCommitment flow.StateCommitment
}

// ExportReporter writes data that can be leveraged outside of extraction
type ExportReporter struct {
	Chain                   flow.Chain
	Log                     zerolog.Logger
	PreviousStateCommitment flow.StateCommitment
}

func (e *ExportReporter) Name() string {
	return "ExportReporter"
}

func (e *ExportReporter) Report(payload []ledger.Payload, o ledger.ExportOutputs) error {
	script, _, err := ExecuteCurrentEpochScript(e.Chain, payload)

	if err == nil && script.Err == nil && script.Value != nil {
		epochCounter := script.Value.ToGoValue().(uint64)
		e.Log.
			Info().
			Uint64("epochCounter", epochCounter).
			Msg("Fetched epoch counter from the FlowEpoch smart contract")

			report := ExportReport{
				CurrentStateCommitment:  o.CurrentStateCommitement,
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
	} else {
		e.Log.
			Error().
			Err(script.Err).
			Msg("Failed to get epoch counter")
		return nil
	}
}

var _ ledger.Reporter = &ExportReporter{}
