package cmd

import (
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	flagBootDir string
	log         zerolog.Logger
)

var rootCmd = &cobra.Command{
	Use:   "transit",
	Short: "The transit script facilitates node operators with utility commands and uploading their public keys and to Dapper servers",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&flagBootDir, "boot-dir", "b", "./bootstrap", "the bootstrap directory")
	log = zerolog.New(zerolog.NewConsoleWriter())
	cobra.OnInitialize(initConfig)
}

func initConfig() {
	viper.AutomaticEnv()
}
