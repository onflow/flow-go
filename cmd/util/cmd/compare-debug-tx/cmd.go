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
	flagShowTraceDiff       bool
	flagBlockIDs            string
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

	Cmd.Flags().BoolVar(&flagShowTraceDiff, "show-trace-diff", false, "show trace diff output (default: false)")

	Cmd.Flags().StringVar(&flagBlockIDs, "block-ids", "", "comma-separated hex block IDs (alternative to --block-id + --block-count)")
}

// Config holds the parameters for a compare-debug-tx invocation.
type Config struct {
	Branch1             string
	Branch2             string
	Chain               string
	AccessAddress       string
	ExecutionAddress    string
	ComputeLimit        uint64
	UseExecutionDataAPI bool
	BlockIDs            []string // pre-resolved block IDs (hex strings)
	TxIDs               []string // positional transaction IDs
	Parallel            int
	LogCadenceTraces    bool
	OnlyTraceCadence    bool
	EntropyProvider     string
	ShowTraceDiff       bool
}

// closeFile closes the file and fatals on failure.
func closeFile(f *os.File) {
	if err := f.Close(); err != nil {
		log.Fatal().Err(err).Str("file", f.Name()).Msg("failed to close temp file")
	}
}

// removeFile removes the file at path and fatals on failure.
func removeFile(path string) {
	if err := os.Remove(path); err != nil {
		log.Fatal().Err(err).Str("file", path).Msg("failed to remove temp file")
	}
}

func run(_ *cobra.Command, args []string) {
	cfg := Config{
		Branch1:             flagBranch1,
		Branch2:             flagBranch2,
		Chain:               flagChain,
		AccessAddress:       flagAccessAddress,
		ExecutionAddress:    flagExecutionAddress,
		ComputeLimit:        flagComputeLimit,
		UseExecutionDataAPI: flagUseExecutionDataAPI,
		TxIDs:               args,
		Parallel:            flagParallel,
		LogCadenceTraces:    flagLogCadenceTraces,
		OnlyTraceCadence:    flagOnlyTraceCadence,
		EntropyProvider:     flagEntropyProvider,
		ShowTraceDiff:       flagShowTraceDiff,
	}

	// Resolve block IDs before git checkouts.
	if flagBlockIDs != "" {
		if flagBlockID != "" || flagBlockCount > 1 {
			log.Fatal().Msg("--block-ids cannot be used with --block-id or --block-count > 1")
		}
		if len(args) > 0 {
			log.Fatal().Msg("--block-ids cannot be used with positional transaction IDs")
		}
		for _, raw := range strings.Split(flagBlockIDs, ",") {
			raw = strings.TrimSpace(raw)
			if _, err := flow.HexStringToIdentifier(raw); err != nil {
				log.Fatal().Err(err).Str("id", raw).Msg("invalid block ID in --block-ids")
			}
			cfg.BlockIDs = append(cfg.BlockIDs, raw)
		}
	} else if flagBlockCount > 0 {
		if flagBlockID == "" {
			log.Fatal().Msg("--block-count requires --block-id to be set")
		}
		if len(args) > 0 {
			log.Fatal().Msg("--block-count cannot be used with positional transaction IDs")
		}
		cfg.BlockIDs = resolveBlockChain(flagAccessAddress, flagBlockID, flagBlockCount)
	}

	Run(cfg)
}

// Run executes the compare-debug-tx workflow with the given configuration.
func Run(cfg Config) {
	repoRoot := findRepoRoot()
	checkCleanWorkingTree(repoRoot)

	result1, err := os.CreateTemp("", "compare-debug-tx-result1-*")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create temp file for result1")
	}
	defer removeFile(result1.Name())
	defer closeFile(result1)

	result2, err := os.CreateTemp("", "compare-debug-tx-result2-*")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create temp file for result2")
	}
	defer removeFile(result2.Name())
	defer closeFile(result2)

	trace1, err := os.CreateTemp("", "compare-debug-tx-trace1-*")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create temp file for trace1")
	}
	defer removeFile(trace1.Name())
	defer closeFile(trace1)

	trace2, err := os.CreateTemp("", "compare-debug-tx-trace2-*")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create temp file for trace2")
	}
	defer removeFile(trace2.Name())
	defer closeFile(trace2)

	checkoutBranch(repoRoot, cfg.Branch1)
	binary1 := buildUtil(repoRoot)
	defer removeFile(binary1)
	perBlock1 := runAllBlocks(cfg, binary1, result1, trace1)
	defer cleanupPerBlockFiles(perBlock1)

	checkoutBranch(repoRoot, cfg.Branch2)
	binary2 := buildUtil(repoRoot)
	defer removeFile(binary2)
	perBlock2 := runAllBlocks(cfg, binary2, result2, trace2)
	defer cleanupPerBlockFiles(perBlock2)

	if len(cfg.BlockIDs) == 1 {
		fmt.Printf("=== Result diff (%s vs %s) ===\n", cfg.Branch1, cfg.Branch2)
		diffFiles(result1.Name(), result2.Name(), cfg.Branch1, cfg.Branch2)

		if cfg.ShowTraceDiff {
			fmt.Printf("=== Trace diff (%s vs %s) ===\n", cfg.Branch1, cfg.Branch2)
			diffFiles(trace1.Name(), trace2.Name(), cfg.Branch1, cfg.Branch2)
		}
	}

	printMismatchStats(cfg.BlockIDs, perBlock1, perBlock2)
}

// perBlockFiles holds per-block result and trace temp files.
type perBlockFiles struct {
	result *os.File
	trace  *os.File
}

// cleanupPerBlockFiles closes and removes all per-block temp files.
func cleanupPerBlockFiles(files []perBlockFiles) {
	for _, f := range files {
		closeFile(f.result)
		removeFile(f.result.Name())
		closeFile(f.trace)
		removeFile(f.trace.Name())
	}
}

// runAllBlocks runs debug-tx for each block ID in cfg.BlockIDs (or a single invocation when
// BlockIDs is empty), up to cfg.Parallel blocks concurrently. Results and traces from each
// block are collected into per-block temp files and concatenated into resultDst and traceDst
// in deterministic order after all blocks complete.
//
// When BlockIDs has entries, the per-block files are returned and the caller is responsible
// for cleanup via cleanupPerBlockFiles. When BlockIDs is empty, nil is returned.
func runAllBlocks(cfg Config, binaryPath string, resultDst *os.File, traceDst *os.File) []perBlockFiles {
	if len(cfg.BlockIDs) == 0 {
		if err := runDebugTx(binaryPath, buildDebugTxArgs(cfg, traceDst.Name(), ""), resultDst); err != nil {
			log.Fatal().Err(err).Msg("failed to run debug-tx")
		}
		return nil
	}

	files := make([]perBlockFiles, len(cfg.BlockIDs))
	for i := range cfg.BlockIDs {
		result, err := os.CreateTemp("", "compare-debug-tx-block-result-*")
		if err != nil {
			log.Fatal().Err(err).Msg("failed to create per-block result temp file")
		}
		trace, err := os.CreateTemp("", "compare-debug-tx-block-trace-*")
		if err != nil {
			log.Fatal().Err(err).Msg("failed to create per-block trace temp file")
		}
		files[i] = perBlockFiles{result: result, trace: trace}
	}

	g, _ := errgroup.WithContext(context.Background())
	g.SetLimit(cfg.Parallel)

	for i, blockID := range cfg.BlockIDs {
		g.Go(func() error {
			return runDebugTx(binaryPath, buildDebugTxArgs(cfg, files[i].trace.Name(), blockID), files[i].result)
		})
	}
	if err := g.Wait(); err != nil {
		log.Fatal().Err(err).Msg("failed to run debug-tx")
	}

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

	return files
}

// resolveBlockChain fetches count consecutive block IDs starting from startBlockID,
// following parent IDs, and returns them as hex strings.
func resolveBlockChain(accessAddress string, startBlockID string, count int) []string {
	blockID, err := flow.HexStringToIdentifier(startBlockID)
	if err != nil {
		log.Fatal().Err(err).Str("ID", startBlockID).Msg("failed to parse block ID")
	}

	config, err := grpcclient.NewFlowClientConfig(accessAddress, "", flow.ZeroID, true)
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

// buildUtil compiles `./cmd/util` with the cadence_tracing build tag in repoRoot,
// writes the binary to a temp file, and returns its path. The caller is responsible
// for removing the file when done.
func buildUtil(repoRoot string) string {
	binary, err := os.CreateTemp("", "compare-debug-tx-util-*")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create temp file for util binary")
	}
	closeFile(binary)

	cmd := exec.Command("go", "build", "-tags", "cadence_tracing", "-o", binary.Name(), "./cmd/util")
	cmd.Dir = repoRoot
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatal().Err(err).Msg("failed to build util binary")
	}

	return binary.Name()
}

// runDebugTx runs the `debug-tx` subcommand of the prebuilt binaryPath with fwdArgs,
// directing stdout to resultDst and stderr to os.Stderr.
func runDebugTx(binaryPath string, fwdArgs []string, resultDst *os.File) error {
	cmd := exec.Command(binaryPath, append([]string{"debug-tx"}, fwdArgs...)...)
	cmd.Stdout = resultDst
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// buildDebugTxArgs assembles the flag arguments for the debug-tx command.
// blockIDOverride, when non-empty, is passed as --block-id.
func buildDebugTxArgs(cfg Config, tracePath string, blockIDOverride string) []string {
	args := []string{
		"--chain=" + cfg.Chain,
		"--access-address=" + cfg.AccessAddress,
		fmt.Sprintf("--compute-limit=%d", cfg.ComputeLimit),
		fmt.Sprintf("--use-execution-data-api=%t", cfg.UseExecutionDataAPI),
		fmt.Sprintf("--log-cadence-traces=%t", cfg.LogCadenceTraces),
		fmt.Sprintf("--only-trace-cadence=%t", cfg.OnlyTraceCadence),
		"--entropy-provider=" + cfg.EntropyProvider,
		"--show-result=true",
		"--trace=" + tracePath,
	}

	if cfg.ExecutionAddress != "" {
		args = append(args, "--execution-address="+cfg.ExecutionAddress)
	}

	if blockIDOverride != "" {
		args = append(args, "--block-id="+blockIDOverride)
	}

	args = append(args, cfg.TxIDs...)
	return args
}

// printMismatchStats compares per-block result files from two branches and prints
// statistics on how many blocks and transactions had result mismatches.
func printMismatchStats(blockIDs []string, files1, files2 []perBlockFiles) {
	blocksWithMismatch := 0
	totalTxs := 0
	txsWithMismatch := 0

	fmt.Printf("\n=== Result Mismatch Summary ===\n")

	for i, blockID := range blockIDs {
		data1 := readFileFromStart(files1[i].result)
		data2 := readFileFromStart(files2[i].result)

		sections1 := splitTxSections(data1)
		sections2 := splitTxSections(data2)

		blockTxMismatches := 0
		txCount := max(len(sections1), len(sections2))
		totalTxs += txCount

		for j := range txCount {
			var s1, s2 []byte
			if j < len(sections1) {
				s1 = sections1[j]
			}
			if j < len(sections2) {
				s2 = sections2[j]
			}
			if !bytes.Equal(s1, s2) {
				txsWithMismatch++
				blockTxMismatches++
			}
		}

		if blockTxMismatches > 0 {
			blocksWithMismatch++
			log.Error().Msgf("Block %s: %d/%d transactions differ", blockID, blockTxMismatches, txCount)
		}
	}

	log.Info().Msgf("Blocks with result mismatches: %d/%d", blocksWithMismatch, len(blockIDs))
	log.Info().Msgf("Transactions with result mismatches: %d/%d", txsWithMismatch, totalTxs)
}

// readFileFromStart seeks to the beginning of the file and reads all content.
func readFileFromStart(f *os.File) []byte {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		log.Fatal().Err(err).Msg("failed to seek file")
	}
	data, err := io.ReadAll(f)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to read file")
	}
	return data
}

// splitTxSections splits result data into per-transaction sections.
// Each section starts with a "# ID: " line and includes all content up to the next such line.
func splitTxSections(data []byte) [][]byte {
	prefix := []byte("# ID: ")
	var sections [][]byte
	sectionStart := -1

	for i := 0; i < len(data); {
		lineEnd := bytes.IndexByte(data[i:], '\n')
		var line []byte
		if lineEnd < 0 {
			line = data[i:]
		} else {
			line = data[i : i+lineEnd]
		}

		if bytes.HasPrefix(line, prefix) {
			if sectionStart >= 0 {
				sections = append(sections, data[sectionStart:i])
			}
			sectionStart = i
		}

		if lineEnd < 0 {
			break
		}
		i += lineEnd + 1
	}
	if sectionStart >= 0 {
		sections = append(sections, data[sectionStart:])
	}
	return sections
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
