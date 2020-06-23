package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/dapperlabs/flow-go/cmd/util/cmd/common"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/metrics"
	bstorage "github.com/dapperlabs/flow-go/storage/badger"
)

var (
	flagHeight           uint64
	flagAllowUnfinalized bool
	flagAllowUnsealed    bool
	flagDatadir          string
)

// blockHashByHeight retreives the block hash by height
func blockHashByHeight(_ *cobra.Command, _ []string) {
	db := common.InitStorage(flagDatadir)
	cache := metrics.NewCacheCollector(flow.Mainnet)
	headers := bstorage.NewHeaders(cache, db)
	seals := bstorage.NewSeals(cache, db)

	var h *flow.Header

	if !flagAllowUnfinalized {
		// verify finalized
		// retrieve the block at the desired height + 3, to ensure the height is finalized
		var err error
		h, err = headers.ByHeight(flagHeight + 3)
		if err != nil {
			log.Fatal().Err(err).Msgf("block at height %v not yet finalized", flagHeight)
		}

		for i := 2; i >= 0; i-- {
			parentID := h.ParentID
			h, err = headers.ByBlockID(parentID)
			if err != nil {
				log.Fatal().Err(err).Msgf("could not get header at height %v with ID %v", flagHeight+uint64(i),
					parentID)
			}
		}
	} else {
		// verify exists
		var err error
		h, err = headers.ByHeight(flagHeight)
		if err != nil {
			log.Fatal().Err(err).Msgf("could not get header at height %v", flagHeight)
		}
	}

	if !flagAllowUnsealed {
		// verify sealed
		_, err := seals.ByBlockID(h.ID())
		if err != nil {
			log.Fatal().Err(err).Msgf("block at height %v not yet sealed", flagHeight)
		}
	}

	fmt.Println(h.ID())
}

var blockHashByHeightCmd = &cobra.Command{
	Use:   "block-hash-by-height",
	Short: "Retrieve the block hash for the finalized block at the given height",
	Run:   blockHashByHeight,
}

func init() {
	rootCmd.AddCommand(blockHashByHeightCmd)

	blockHashByHeightCmd.Flags().Uint64Var(&flagHeight, "height", 0,
		"height for which the block hash should be retrieved")
	_ = blockHashByHeightCmd.MarkFlagRequired("height")

	blockHashByHeightCmd.Flags().BoolVar(&flagAllowUnfinalized, "allow-unfinalized", false,
		"allows retrieval of hashes of unfinalized blocks. Be careful, these could be ambiguous. Defaults to false.")

	blockHashByHeightCmd.Flags().BoolVar(&flagAllowUnsealed, "allow-unsealed", false,
		"allows retrieval of hashes of unsealed blocks. Defaults to false.")

	homedir, _ := os.UserHomeDir()
	datadir := filepath.Join(homedir, ".flow", "database")
	blockHashByHeightCmd.Flags().StringVar(&flagDatadir, "datadir", datadir,
		"directory that stores the protocol state")
}
