package compare_debug_tx

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/grpcclient"
	"github.com/onflow/flow-go/utils/debug"
)

var (
	flagBranch1             string
	flagBranch2             string
	flagChain               string
	flagAccessAddress       string
	flagExecutionAddress    string
	flagComputeLimit        uint64
	flagUseExecutionDataAPI bool
	flagBlockID             string
	flagBlockCount          int
	flagParallel            int
	flagLogCadenceTraces    bool
	flagOnlyTraceCadence    bool
	flagEntropyProvider     string
)

var Cmd = &cobra.Command{
	Use:   "compare-debug-tx",
	Short: "compare transaction execution between two git branches",
	Run:   run,
}

func init() {
	Cmd.Flags().StringVar(&flagBranch1, "branch1", "", "first git branch (required)")
	_ = Cmd.MarkFlagRequired("branch1")

	Cmd.Flags().StringVar(&flagBranch2, "branch2", "", "second git branch (required)")
	_ = Cmd.MarkFlagRequired("branch2")

	Cmd.Flags().StringVar(&flagChain, "chain", "", "Chain name")
	_ = Cmd.MarkFlagRequired("chain")

	Cmd.Flags().StringVar(&flagAccessAddress, "access-address", "", "address of the access node")
	_ = Cmd.MarkFlagRequired("access-address")

	Cmd.Flags().StringVar(&flagExecutionAddress, "execution-address", "", "address of the execution node (required if --use-execution-data-api is false)")

	Cmd.Flags().Uint64Var(&flagComputeLimit, "compute-limit", flow.DefaultMaxTransactionGasLimit, "transaction compute limit")

	Cmd.Flags().BoolVar(&flagUseExecutionDataAPI, "use-execution-data-api", true, "use the execution data API (default: true)")

	Cmd.Flags().StringVar(&flagBlockID, "block-id", "", "block ID")

	Cmd.Flags().IntVar(&flagBlockCount, "block-count", 1, "number of consecutive blocks to process (default: 1); if > 1, requires --block-id and no positional transaction IDs")

	Cmd.Flags().IntVar(&flagParallel, "parallel", 1, "number of blocks to process in parallel (default: 1)")

	Cmd.Flags().BoolVar(&flagLogCadenceTraces, "log-cadence-traces", false, "log Cadence traces (default: false)")

	Cmd.Flags().BoolVar(&flagOnlyTraceCadence, "only-trace-cadence", false, "when tracing, only include spans related to Cadence execution (default: false)")

	Cmd.Flags().StringVar(&flagEntropyProvider, "entropy-provider", "none", "entropy provider to use (default: none; options: none, block-hash)")
}

func run(_ *cobra.Command, args []string) {
	repoRoot := findRepoRoot()
	checkCleanWorkingTree(repoRoot)

	// Resolve block IDs before git checkouts when --block-count > 1.
	var blockIDs []string
	if flagBlockCount != 1 {
		if flagBlockID == "" {
			log.Fatal().Msg("--block-count requires --block-id to be set")
		}
		if len(args) > 0 {
			log.Fatal().Msg("--block-count cannot be used with positional transaction IDs")
		}
		blockIDs = resolveBlockChain(flagBlockID, flagBlockCount)
	}

	result1, err := os.CreateTemp("", "compare-debug-tx-result1-*")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create temp file for result1")
	}
	defer os.Remove(result1.Name())
	defer result1.Close()

	result2, err := os.CreateTemp("", "compare-debug-tx-result2-*")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create temp file for result2")
	}
	defer os.Remove(result2.Name())
	defer result2.Close()

	trace1, err := os.CreateTemp("", "compare-debug-tx-trace1-*")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create temp file for trace1")
	}
	defer os.Remove(trace1.Name())
	defer trace1.Close()

	trace2, err := os.CreateTemp("", "compare-debug-tx-trace2-*")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create temp file for trace2")
	}
	defer os.Remove(trace2.Name())
	defer trace2.Close()

	checkoutBranch(repoRoot, flagBranch1)
	runAllBlocks(repoRoot, args, blockIDs, result1, trace1)

	checkoutBranch(repoRoot, flagBranch2)
	runAllBlocks(repoRoot, args, blockIDs, result2, trace2)

	fmt.Printf("=== Result diff (%s vs %s) ===\n", flagBranch1, flagBranch2)
	diffFiles(result1.Name(), result2.Name(), flagBranch1, flagBranch2)

	fmt.Printf("=== Trace diff (%s vs %s) ===\n", flagBranch1, flagBranch2)
	diffFiles(trace1.Name(), trace2.Name(), flagBranch1, flagBranch2)
}

// runAllBlocks runs debug-tx for each block ID in blockIDs (or a single invocation when
// blockIDs is empty), up to flagParallel blocks concurrently. Results and traces from each
// block are collected into per-block temp files and concatenated into resultDst and traceDst
// in deterministic order after all blocks complete.
func runAllBlocks(repoRoot string, txIDs []string, blockIDs []string, resultDst *os.File, traceDst *os.File) {
	if len(blockIDs) == 0 {
		runDebugTx(repoRoot, buildDebugTxArgs(txIDs, traceDst.Name(), ""), resultDst)
		return
	}

	type blockFiles struct {
		result *os.File
		trace  *os.File
	}

	files := make([]blockFiles, len(blockIDs))
	for i := range blockIDs {
		result, err := os.CreateTemp("", "compare-debug-tx-block-result-*")
		if err != nil {
			log.Fatal().Err(err).Msg("failed to create per-block result temp file")
		}
		trace, err := os.CreateTemp("", "compare-debug-tx-block-trace-*")
		if err != nil {
			log.Fatal().Err(err).Msg("failed to create per-block trace temp file")
		}
		files[i] = blockFiles{result: result, trace: trace}
	}
	defer func() {
		for _, f := range files {
			f.result.Close()
			os.Remove(f.result.Name())
			f.trace.Close()
			os.Remove(f.trace.Name())
		}
	}()

	g, _ := errgroup.WithContext(context.Background())
	g.SetLimit(flagParallel)

	for i, blockID := range blockIDs {
		g.Go(func() error {
			runDebugTx(repoRoot, buildDebugTxArgs(txIDs, files[i].trace.Name(), blockID), files[i].result)
			return nil
		})
	}
	_ = g.Wait()

	for _, f := range files {
		if _, err := f.result.Seek(0, io.SeekStart); err != nil {
			log.Fatal().Err(err).Msg("failed to seek per-block result file")
		}
		if _, err := io.Copy(resultDst, f.result); err != nil {
			log.Fatal().Err(err).Msg("failed to append per-block result")
		}
		if _, err := f.trace.Seek(0, io.SeekStart); err != nil {
			log.Fatal().Err(err).Msg("failed to seek per-block trace file")
		}
		if _, err := io.Copy(traceDst, f.trace); err != nil {
			log.Fatal().Err(err).Msg("failed to append per-block trace")
		}
	}
}

// resolveBlockChain fetches count consecutive block IDs starting from startBlockID,
// following parent IDs, and returns them as hex strings.
func resolveBlockChain(startBlockID string, count int) []string {
	blockID, err := flow.HexStringToIdentifier(startBlockID)
	if err != nil {
		log.Fatal().Err(err).Str("ID", startBlockID).Msg("failed to parse block ID")
	}

	config, err := grpcclient.NewFlowClientConfig(flagAccessAddress, "", flow.ZeroID, true)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create flow client config")
	}

	flowClient, err := grpcclient.FlowClient(config)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create flow client")
	}

	ctx := context.Background()

	var blockIDs []string
	for range count {
		header, err := debug.GetAccessAPIBlockHeader(ctx, flowClient.RPCClient(), blockID)
		if err != nil {
			log.Fatal().Err(err).Str("blockID", blockID.String()).Msg("failed to fetch block header")
		}

		log.Info().Msgf("Resolved block %s at height %d", blockID, header.Height)
		blockIDs = append(blockIDs, blockID.String())
		blockID = header.ParentID
	}

	return blockIDs
}

// findRepoRoot returns the absolute path to the root of the git repository.
//
// No error returns are expected during normal operation.
func findRepoRoot() string {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to find repo root")
	}
	return strings.TrimSpace(string(out))
}

// checkCleanWorkingTree fatals if the working tree has uncommitted changes (excluding untracked files).
//
// No error returns are expected during normal operation.
func checkCleanWorkingTree(repoRoot string) {
	cmd := exec.Command("git", "status", "--porcelain", "--untracked-files=no")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to check working tree status")
	}
	if len(bytes.TrimSpace(out)) > 0 {
		log.Fatal().Msg("working tree has uncommitted changes; stash or commit before comparing branches")
	}
}

// checkoutBranch checks out the given branch in the repo at repoRoot.
//
// No error returns are expected during normal operation.
func checkoutBranch(repoRoot, branch string) {
	cmd := exec.Command("git", "checkout", branch)
	cmd.Dir = repoRoot
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatal().Err(err).Str("branch", branch).Msg("failed to checkout branch")
	}
}

// runDebugTx runs `go run -tags cadence_tracing ./cmd/util debug-tx` with fwdArgs in repoRoot,
// directing stdout to resultDst and stderr to os.Stderr.
//
// No error returns are expected during normal operation.
func runDebugTx(repoRoot string, fwdArgs []string, resultDst *os.File) {
	goArgs := append([]string{"run", "-tags", "cadence_tracing", "./cmd/util", "debug-tx"}, fwdArgs...)
	cmd := exec.Command("go", goArgs...)
	cmd.Dir = repoRoot
	cmd.Stdout = resultDst
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatal().Err(err).Msg("failed to run debug-tx")
	}
}

// buildDebugTxArgs assembles the flag arguments for the debug-tx command, appending
// --show-result=true, --trace=<tracePath>, and the given tx IDs.
// blockIDOverride, when non-empty, overrides --block-id instead of using flagBlockID.
func buildDebugTxArgs(txIDs []string, tracePath string, blockIDOverride string) []string {
	args := []string{
		"--chain=" + flagChain,
		"--access-address=" + flagAccessAddress,
		fmt.Sprintf("--compute-limit=%d", flagComputeLimit),
		fmt.Sprintf("--use-execution-data-api=%t", flagUseExecutionDataAPI),
		fmt.Sprintf("--log-cadence-traces=%t", flagLogCadenceTraces),
		fmt.Sprintf("--only-trace-cadence=%t", flagOnlyTraceCadence),
		"--entropy-provider=" + flagEntropyProvider,
		"--show-result=true",
		"--trace=" + tracePath,
	}

	if flagExecutionAddress != "" {
		args = append(args, "--execution-address="+flagExecutionAddress)
	}

	blockID := flagBlockID
	if blockIDOverride != "" {
		blockID = blockIDOverride
	}
	if blockID != "" {
		args = append(args, "--block-id="+blockID)
	}

	args = append(args, txIDs...)
	return args
}

// diffFiles runs `diff -u --label label1 --label label2 file1 file2` and prints the output.
// A diff exit code of 1 (files differ) is not treated as an error.
//
// No error returns are expected during normal operation.
func diffFiles(file1, file2, label1, label2 string) {
	cmd := exec.Command("diff", "-u", "--label", label1, "--label", label2, file1, file2)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			// exit code 1 means files differ, which is expected
			return
		}
		log.Fatal().Err(err).Msg("failed to diff files")
	}
}
