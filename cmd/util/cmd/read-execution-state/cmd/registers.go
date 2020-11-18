package cmd

import (
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var registersCmd = &cobra.Command{
	Use:   "registers",
	Short: "",
	Run:   registers,
}

func init() {
	RootCmd.AddCommand(listTriesCmd)
}

func registers(*cobra.Command, []string) {
	startTime := time.Now()

	log.Info().Float64("total_time_s", time.Since(startTime).Seconds()).Msg("finished")
}
