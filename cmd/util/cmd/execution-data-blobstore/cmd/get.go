package cmd

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
)

var (
	flagID string
)

var Cmd = &cobra.Command{
	Use:   "get",
	Short: "Read execution data from the blobstore",
	Run:   run,
}

func init() {
	rootCmd.AddCommand(Cmd)

	Cmd.Flags().StringVar(&flagID, "id", "", "Execution data ID")
}

func run(*cobra.Command, []string) {
	bs, ds := initBlobstore()
	defer ds.Close()

	logger := zerolog.New(os.Stdout)

	eds := execution_data.NewExecutionDataStore(bs, execution_data.DefaultSerializer)

	b, err := hex.DecodeString(flagID)
	if err != nil {
		logger.Fatal().Err(err).Msg("invalid execution data ID")
	}

	edID := flow.HashToID(b)

	ed, err := eds.Get(context.Background(), edID)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to get execution data")
	}

	bytes, err := json.MarshalIndent(ed, "", "  ")
	if err != nil {
		logger.Fatal().Err(err).Msg("could not marshal execution data into json")
	}

	fmt.Println(string(bytes))
}
