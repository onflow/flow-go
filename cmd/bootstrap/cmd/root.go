package cmd

import (
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var outdir string

var rootCmd = &cobra.Command{
	Use:   "bootstrap",
	Short: "Bootstrap a flow network",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&outdir, "outdir", "o", "bootstrap", "output directory for generated files")

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	cobra.OnInitialize(initConfig)
}

func initConfig() {
	viper.AutomaticEnv()
}
