package find

import (
	"encoding/json"
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/dapperlabs/flow-go/cmd/util/cmd/common"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/storage/badger"
)

var (
	flagBlockHeight uint64
	flagDatadir     string
)

var Cmd = &cobra.Command{
	Use:   "find-block",
	Short: "Finds a block matching given parameters",
	Run:   run,
}

func init() {

	Cmd.Flags().StringVar(&flagDatadir, "datadir", "",
		"directory that stores the protocol state")
	_ = Cmd.MarkFlagRequired("datadir")

	Cmd.Flags().Uint64Var(&flagBlockHeight, "block-height", 0,
		"block height")
	_ = Cmd.MarkFlagRequired("block-height")
}

func run(*cobra.Command, []string) {

	db := common.InitStorage(flagDatadir)
	defer db.Close()

	cache := &metrics.NoopCollector{}

	headers := badger.NewHeaders(cache, db)

	foundHeaders, err := headers.FindHeaders(func(header *flow.Header) bool {
		return header.Height == flagBlockHeight
	})
	if err != nil {
		log.Fatal().Err(err).Msg("error while traversing blocks")
	}

	for _, header := range foundHeaders {
		fmt.Printf("Block ID %s:\n", header.ID().String())
		bytes, err := json.MarshalIndent(header, "", "  ")
		if err != nil {
			log.Fatal().Err(err).Msg("error while marshalling found block")
		}
		fmt.Println(string(bytes))
	}
}
