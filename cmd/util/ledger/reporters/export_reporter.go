package reporters

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-core-contracts/lib/go/templates"

	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/systemcontracts"
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
	getAfterMigrationSCFunc  GetStateCommitmentFunc
}

func NewExportReporter(
	logger zerolog.Logger,
	chain flow.Chain,
	getBeforeMigrationSCFunc GetStateCommitmentFunc,
	getAfterMigrationSCFunc GetStateCommitmentFunc,
) *ExportReporter {
	return &ExportReporter{
		logger:                   logger,
		chain:                    chain,
		getBeforeMigrationSCFunc: getBeforeMigrationSCFunc,
		getAfterMigrationSCFunc:  getAfterMigrationSCFunc,
	}
}

func (e *ExportReporter) Name() string {
	return "ExportReporter"
}

func (e *ExportReporter) Report(payload []ledger.Payload) error {
	script, _, err := ExecuteCurrentEpochScript(e.chain, payload)
	failedExportReport := ExportReport{
		ReportSucceeded: true,
	}
	failedReport, _ := json.MarshalIndent(failedExportReport, "", " ")

	if err != nil {
		e.logger.
			Error().
			Err(err).
			Msg("error running GetCurrentEpochCounter script")

		_ = ioutil.WriteFile("export_report.json", failedReport, 0644)
		// Safely exit and move on to next reporter so we do not block other reporters
		return nil
	}

	if script.Err != nil && script.Value == nil {
		_ = ioutil.WriteFile("export_report.json", failedReport, 0644)
		e.logger.
			Error().
			Err(script.Err).
			Msg("Failed to get epoch counter")

		// Safely exit and move on to next reporter so we do not block other reporters
		return nil
	}

	epochCounter := script.Value.ToGoValue().(uint64)
	e.logger.
		Info().
		Uint64("epochCounter", epochCounter).
		Msg("Fetched epoch counter from the FlowEpoch smart contract")

	report := ExportReport{
		EpochCounter:            script.Value.ToGoValue().(uint64),
		PreviousStateCommitment: e.getBeforeMigrationSCFunc(),
		CurrentStateCommitment:  e.getAfterMigrationSCFunc(),
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

// Executes script to get current epoch from chain
func ExecuteCurrentEpochScript(c flow.Chain, payload []ledger.Payload) (*fvm.ScriptProcedure, flow.Address, error) {
	l := migrations.NewView(payload)
	prog := programs.NewEmptyPrograms()
	vm := fvm.NewVirtualMachine(fvm.NewInterpreterRuntime())
	ctx := fvm.NewContext(zerolog.Nop(), fvm.WithChain(c))

	sc, err := systemcontracts.SystemContractsForChain(c.ChainID())
	if err != nil {
		return nil, flow.Address{}, fmt.Errorf("error getting SystemContracts for chain %s: %w", c.String(), err)
	}
	address := sc.Epoch.Address
	scriptCode := templates.GenerateGetCurrentEpochCounterScript(templates.Environment{
		EpochAddress: address.Hex(),
	})
	script := fvm.Script(scriptCode)
	return script, address, vm.Run(ctx, script, l, prog)
}
