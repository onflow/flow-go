package recalculate_mappings

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	extract "github.com/onflow/flow-go/cmd/util/cmd/execution-state-extract"
	read_mappings "github.com/onflow/flow-go/cmd/util/cmd/read-mappings"
	"github.com/onflow/flow-go/engine/execution/state/delta"
)

var (
	flagOutputFile    string
	flagMappingInputs []string
)

var Cmd = &cobra.Command{
	Use:   "recalculate-mappings",
	Short: "Join and recalculate mappings",
	Run:   run,
}

func init() {

	Cmd.Flags().StringVar(&flagOutputFile, "mappings-file", "",
		"file to write mappings to")

	Cmd.Flags().StringArrayVar(&flagMappingInputs, "input", nil,
		"file(s) to read mappings from")

	_ = Cmd.MarkFlagRequired("mappings-file")
	_ = Cmd.MarkFlagRequired("input")
}

func run(*cobra.Command, []string) {

	mappings := make(map[string]delta.Mapping)

	for _, input := range flagMappingInputs {
		readMappings, err := extract.ReadMegamappings(input)
		if err != nil {
			log.Fatal().Err(err).Msg("cannot read mappings")
		}
		fixedMappings, err := extract.FixMappings(readMappings)
		if err != nil {
			log.Fatal().Err(err).Msgf("cannot fix mappings from %s", input)
		}
		for k, v := range fixedMappings {
			mappings[k] = v
		}
	}

	err := read_mappings.WriteMegamappings(mappings, flagOutputFile)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot save mappings to a file")
	}
}
