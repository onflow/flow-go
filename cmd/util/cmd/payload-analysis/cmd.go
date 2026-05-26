package payload_analysis

import (
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "payload-analysis",
	Short: "Analyze payload sizes in WAL files or chunk data packs",
	Long: `Analyze payload size distributions to understand performance implications for:
1. Trie-storehouse consistency checks (hashing during proof generation)
2. Full checkpoint generation (scanning and hashing WAL payloads)`,
}

func init() {
	Cmd.AddCommand(walCmd)
	Cmd.AddCommand(cdpCmd)
	Cmd.AddCommand(cdpDedupCmd)
}
