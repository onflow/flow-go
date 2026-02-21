package compare_debug_tx

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/model/flow"
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

	Cmd.Flags().BoolVar(&flagLogCadenceTraces, "log-cadence-traces", false, "log Cadence traces (default: false)")

	Cmd.Flags().BoolVar(&flagOnlyTraceCadence, "only-trace-cadence", false, "when tracing, only include spans related to Cadence execution (default: false)")

	Cmd.Flags().StringVar(&flagEntropyProvider, "entropy-provider", "none", "entropy provider to use (default: none; options: none, block-hash)")
}

func run(_ *cobra.Command, args []string) {
	repoRoot := findRepoRoot()
	checkCleanWorkingTree(repoRoot)

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
	runDebugTx(repoRoot, buildDebugTxArgs(args, trace1.Name()), result1)

	checkoutBranch(repoRoot, flagBranch2)
	runDebugTx(repoRoot, buildDebugTxArgs(args, trace2.Name()), result2)

	fmt.Printf("=== Result diff (%s vs %s) ===\n", flagBranch1, flagBranch2)
	diffFiles(result1.Name(), result2.Name(), flagBranch1, flagBranch2)

	fmt.Printf("=== Trace diff (%s vs %s) ===\n", flagBranch1, flagBranch2)
	diffFiles(trace1.Name(), trace2.Name(), flagBranch1, flagBranch2)
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
func buildDebugTxArgs(txIDs []string, tracePath string) []string {
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

	if flagBlockID != "" {
		args = append(args, "--block-id="+flagBlockID)
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
