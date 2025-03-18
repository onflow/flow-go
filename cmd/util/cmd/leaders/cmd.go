package leaders

import (
	"encoding/json"
	"os"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/consensus/hotstuff/committees/leader"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol/inmem"
)

var (
	flagStartView uint64
	flagEndView   uint64
)
var Cmd = &cobra.Command{
	Use:   "leaders",
	Short: "Get leader selection for a view range.",
	Long: `Get leader selection for a view range in the current epoch for a provided snapshot.
 Expects a valid protocol state snapshot JSON to be piped into STDIN. Writes a JSON list of leaders for the given view range to STDOUT.`,
	Run: run,
}

func init() {

	Cmd.Flags().Uint64Var(&flagStartView, "start-view", 0, "the inclusive first view to get leader selection for")
	Cmd.Flags().Uint64Var(&flagEndView, "end-view", 0, "the inclusive last view to get leader selection for")
	Cmd.MarkFlagRequired("start-view")
	Cmd.MarkFlagRequired("end-view")
}

func run(*cobra.Command, []string) {

	var snapshot inmem.EncodableSnapshot
	err := json.NewDecoder(os.Stdin).Decode(&snapshot)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to read snapshot from stdin")
	}

	snap := inmem.SnapshotFromEncodable(snapshot)
	epoch, err := snap.Epochs().Current()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to read current epoch")
	}

	// Should match https://github.com/onflow/flow-go/blob/48b6db32d4491903aa0ffa541377c8f239da3bcc/consensus/hotstuff/committees/consensus_committee.go#L74-L78
	selection, err := leader.SelectionForConsensus(
		epoch.InitialIdentities(),
		epoch.RandomSource(),
		epoch.FirstView(),
		epoch.FinalView(),
	)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to read current leader selection")
	}

	type LeaderForView struct {
		View     uint64
		LeaderID flow.Identifier
	}

	leaders := make([]LeaderForView, 0, flagEndView-flagStartView+1)
	for view := flagStartView; view <= flagEndView; view++ {
		leaderID, err := selection.LeaderForView(view)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to read leader for view")
		}
		leaders = append(leaders, LeaderForView{View: view, LeaderID: leaderID})
	}

	err = json.NewEncoder(os.Stdout).Encode(leaders)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to encode leaders")
	}
}
