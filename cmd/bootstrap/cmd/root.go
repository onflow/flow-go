package cmd

import (
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	flagOutdir string
	log        zerolog.Logger
)

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
	rootCmd.PersistentFlags().StringVarP(&flagOutdir, "outdir", "o", "bootstrap",
		"output directory for generated files")

	log = zerolog.New(zerolog.NewConsoleWriter())

	cobra.OnInitialize(initConfig)
}

func initConfig() {
	viper.AutomaticEnv()
}
