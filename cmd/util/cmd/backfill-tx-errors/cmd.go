package backfill_tx_errors

import (
	"strings"
	"unicode/utf8"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/storage/operation/badgerimpl"
	"github.com/onflow/flow-go/storage/store"
)

var (
	flagAccessDatadir    string
	flagExecutionDatadir string
	flagStartHeight      uint64
	flagEndHeight        uint64
	flagExecutionNodeID  string
)

// this command backfills transaction error messages in an Access Node's badger database from an
// Execution Node's badger database. This is much more efficient than using the typical grpc backfill
// process.
var Cmd = &cobra.Command{
	Use:   "backfill-tx-errors",
	Short: "Backfill transaction error messages from an Execution Node's badger database to an Access Node's",
	Run:   run,
}

func init() {
	Cmd.PersistentFlags().StringVar(&flagAccessDatadir, "access-badger-dir", "", "directory to the Access Node's badger database")
	Cmd.PersistentFlags().StringVar(&flagExecutionDatadir, "execution-badger-dir", "", "directory to the Execution Node's badger database")
	Cmd.PersistentFlags().Uint64Var(&flagStartHeight, "start-height", 0, "start height to backfill from")
	Cmd.PersistentFlags().Uint64Var(&flagEndHeight, "end-height", 0, "end height to backfill to")
	Cmd.PersistentFlags().StringVar(&flagExecutionNodeID, "execution-node-id", "", "node id of the execution node whose badger database is used for backfilling")
	_ = Cmd.MarkPersistentFlagRequired("access-badger-dir")
	_ = Cmd.MarkPersistentFlagRequired("execution-badger-dir")
	_ = Cmd.MarkPersistentFlagRequired("execution-node-id")
}

func run(*cobra.Command, []string) {
	if flagAccessDatadir == "" || flagExecutionDatadir == "" {
		log.Fatal().Msg("Both --access-badger-dir and --execution-badger-dir must be provided")
	}

	if flagExecutionNodeID == "" {
		log.Fatal().Msg("--execution-node-id must be provided")
	}

	executionNodeID, err := flow.HexStringToIdentifier(flagExecutionNodeID)
	if err != nil {
		log.Fatal().Err(err).Msg("could not parse execution node id")
	}

	accessDB := common.InitStorage(flagAccessDatadir)
	defer accessDB.Close()

	executionDB := common.InitStorage(flagExecutionDatadir)
	defer executionDB.Close()

	accessStorages := common.InitStorages(accessDB)
	executionStorages := common.InitStorages(executionDB)

	accessTxErrorMessages := store.NewTransactionResultErrorMessages(metrics.NewNoopCollector(), badgerimpl.ToDB(accessDB), 1000)
	executionTxResults := store.NewTransactionResults(metrics.NewNoopCollector(), badgerimpl.ToDB(executionDB), 1000)

	accessBlocks := accessStorages.Blocks
	executionBlocks := executionStorages.Blocks

	accessState, err := common.InitProtocolState(accessDB, accessStorages)
	if err != nil {
		log.Fatal().Err(err).Msg("could not init access protocol state")
	}

	executionState, err := common.InitProtocolState(executionDB, executionStorages)
	if err != nil {
		log.Fatal().Err(err).Msg("could not init execution protocol state")
	}

	root := accessState.Params().SealedRoot()
	final, err := accessState.Final().Head()
	if err != nil {
		log.Fatal().Err(err).Msg("could not get final header from protocol state")
	}

	startHeight := root.Height + 1
	if flagStartHeight > 0 {
		if flagStartHeight <= root.Height {
			log.Fatal().Msgf("start height must be greater than root height %d", root.Height)
		}
		startHeight = flagStartHeight
	}

	endHeight := final.Height
	if flagEndHeight > 0 {
		if flagEndHeight > final.Height {
			log.Fatal().Msgf("end height must be less than or equal to final height %d", final.Height)
		}
		endHeight = flagEndHeight
	}

	executionRoot := executionState.Params().SealedRoot()
	if startHeight < executionRoot.Height {
		log.Fatal().Msgf("start height must be greater than or equal to execution node's root height %d", executionRoot.Height)
	}

	progress := util.LogProgress(log.Logger,
		util.DefaultLogProgressConfig("backfilling", int(endHeight-startHeight+1)),
	)

	for h := startHeight; h <= endHeight; h++ {
		block, err := accessBlocks.ByHeight(h)
		if err != nil {
			log.Fatal().Err(err).Msgf("could not get block at height %d from access node", h)
		}
		blockID := block.ID()

		// if the EN was dynamically bootstrapped from a higher height than the AN, the user needs to
		// explicitly pass a start height that both nodes have.
		if _, err := executionBlocks.ByID(blockID); err != nil {
			log.Fatal().Err(err).Msgf("could not get block at height %d from execution node", h)
		}

		exists, err := accessTxErrorMessages.Exists(blockID)
		if err != nil {
			log.Fatal().Err(err).Msgf("could not check if transaction error messages exist at height %d", h)
		}
		if exists {
			progress(1)
			continue // already indexed, skip block
		}

		txResults, err := executionTxResults.ByBlockID(blockID)
		if err != nil {
			log.Fatal().Err(err).Msgf("could not get transaction results at height %d", h)
		}

		txErrorMessages := make([]flow.TransactionResultErrorMessage, 0)
		for index, result := range txResults {
			if result.ErrorMessage == "" {
				continue // only record failed transactions with error messages
			}

			// replace any non-UTF-8 characters with a question mark. this is usually done by the
			// EN's grpc handler.
			errorMessage := result.ErrorMessage
			if !utf8.ValidString(errorMessage) {
				errorMessage = strings.ToValidUTF8(errorMessage, "?")
			}

			txErrorMessages = append(txErrorMessages, flow.TransactionResultErrorMessage{
				TransactionID: result.TransactionID,
				ErrorMessage:  errorMessage,
				ExecutorID:    executionNodeID,
				Index:         uint32(index),
			})
		}
		if len(txErrorMessages) > 0 {
			accessTxErrorMessages.Store(blockID, txErrorMessages)
		}
		progress(1)
	}

	log.Info().Uint64("start_height", startHeight).Uint64("end_height", endHeight).Msg("indexed transaction error messages")
}
