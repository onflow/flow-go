package ledger_benchmark

import (
	"github.com/spf13/cobra"

	import_checkpoint "github.com/onflow/flow-go/cmd/util/cmd/ledger-benchmark/import-checkpoint"
	read_storehouse "github.com/onflow/flow-go/cmd/util/cmd/ledger-benchmark/read-storehouse"
	merge_wal "github.com/onflow/flow-go/cmd/util/cmd/ledger-benchmark/merge-wal"
	apply_wal "github.com/onflow/flow-go/cmd/util/cmd/ledger-benchmark/apply-wal"
)

var Cmd = &cobra.Command{
	Use:   "ledger-benchmark",
	Short: "Benchmark tools for payloadless trie implementation",
	Long: `A suite of benchmark tools for measuring performance of:
  - Checkpoint import to storehouse (Pebble)
  - Storehouse read performance with concurrent workers
  - WAL file merging at different scales
  - WAL apply to trie performance

These benchmarks help evaluate the feasibility of payloadless trie implementation.`,
}

func init() {
	Cmd.AddCommand(import_checkpoint.Cmd)
	Cmd.AddCommand(read_storehouse.Cmd)
	Cmd.AddCommand(merge_wal.Cmd)
	Cmd.AddCommand(apply_wal.Cmd)
}
