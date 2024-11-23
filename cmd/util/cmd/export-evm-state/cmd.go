package evm_exporter

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/atree"

	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/fvm/evm"
	"github.com/onflow/flow-go/fvm/evm/emulator/state"
	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
)

var (
	flagChain             string
	flagExecutionStateDir string
	flagOutputDir         string
	flagStateCommitment   string
	flagEVMStateGobDir    string
	flagEVMStateGobHeight uint64
)

var Cmd = &cobra.Command{
	Use:   "export-evm-state",
	Short: "exports evm state into a several binary files",
	Run:   run,
}

func init() {
	Cmd.Flags().StringVar(&flagChain, "chain", "", "Chain name")
	_ = Cmd.MarkFlagRequired("chain")

	Cmd.Flags().StringVar(&flagExecutionStateDir, "execution-state-dir", "",
		"Execution Node state dir (where WAL logs are written")

	Cmd.Flags().StringVar(&flagOutputDir, "output-dir", "",
		"Directory to write new Execution State to")
	_ = Cmd.MarkFlagRequired("output-dir")

	Cmd.Flags().StringVar(&flagStateCommitment, "state-commitment", "",
		"State commitment (hex-encoded, 64 characters)")

	Cmd.Flags().StringVar(&flagEVMStateGobDir, "evm_state_gob_dir", "/var/flow/data/evm_state_gob",
		"directory that stores the evm state gob files as checkpoint")

	Cmd.Flags().Uint64Var(&flagEVMStateGobHeight, "evm_state_gob_height", 0,
		"the flow height of the evm state gob files")
}

func run(*cobra.Command, []string) {
	log.Info().Msg("start exporting evm state")
	if flagExecutionStateDir != "" {
		err := ExportEVMState(flagChain, flagExecutionStateDir, flagStateCommitment, flagOutputDir)
		if err != nil {
			log.Fatal().Err(err).Msg("cannot get export evm state")
		}
	} else if flagEVMStateGobDir != "" {
		err := ExportEVMStateFromGob(flagChain, flagEVMStateGobDir, flagEVMStateGobHeight, flagOutputDir)
		if err != nil {
			log.Fatal().Err(err).Msg("cannot get export evm state from gob files")
		}
	}
}

// ExportEVMState evm state
func ExportEVMState(
	chainName string,
	ledgerPath string,
	targetState string,
	outputPath string) error {

	chainID := flow.ChainID(chainName)

	storageRoot := evm.StorageAccountAddress(chainID)
	rootOwner := string(storageRoot.Bytes())

	payloads, err := util.ReadTrie(ledgerPath, util.ParseStateCommitment(targetState))
	if err != nil {
		return err
	}

	// filter payloads of evm storage
	filteredPayloads := make(map[flow.RegisterID]*ledger.Payload, 0)
	for _, payload := range payloads {
		registerID, _, err := convert.PayloadToRegister(payload)
		if err != nil {
			return fmt.Errorf("failed to convert payload to register: %w", err)
		}
		if registerID.Owner == rootOwner {
			filteredPayloads[registerID] = payload
		}
	}

	payloadsLedger := util.NewPayloadsLedger(filteredPayloads)

	return ExportEVMStateFromPayloads(payloadsLedger, storageRoot, outputPath)
}

func ExportEVMStateFromGob(
	chainName string,
	evmStateGobDir string,
	flowHeight uint64,
	outputPath string) error {

	valueFileName, allocatorFileName := evmStateGobFileNamesByEndHeight(evmStateGobDir, flowHeight)
	chainID := flow.ChainID(chainName)

	storageRoot := evm.StorageAccountAddress(chainID)
	valuesGob, err := testutils.DeserializeState(valueFileName)
	if err != nil {
		return err
	}

	allocatorGobs, err := testutils.DeserializeAllocator(allocatorFileName)
	if err != nil {
		return err
	}

	store := testutils.GetSimpleValueStorePopulated(valuesGob, allocatorGobs)

	return ExportEVMStateFromPayloads(store, storageRoot, outputPath)
}

func ExportEVMStateFromPayloads(
	ledger atree.Ledger,
	storageRoot flow.Address,
	outputPath string,
) error {
	exporter, err := state.NewExporter(ledger, storageRoot)
	if err != nil {
		return fmt.Errorf("failed to create exporter: %w", err)
	}

	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		err := os.Mkdir(outputPath, os.ModePerm)
		if err != nil {
			return fmt.Errorf("failed to create path: %w", err)
		}
	}

	fi, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer fi.Close()

	err = exporter.Export(outputPath)
	if err != nil {
		return fmt.Errorf("failed to export: %w", err)
	}
	return nil
}

func evmStateGobFileNamesByEndHeight(evmStateGobDir string, endHeight uint64) (string, string) {
	valueFileName := filepath.Join(evmStateGobDir, fmt.Sprintf("values-%d.gob", endHeight))
	allocatorFileName := filepath.Join(evmStateGobDir, fmt.Sprintf("allocators-%d.gob", endHeight))
	return valueFileName, allocatorFileName
}
