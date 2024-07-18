package update_commitment

import (
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	storagebadger "github.com/onflow/flow-go/storage/badger"
)

var (
	flagDatadir    string
	flagBlockID    string
	flagCommitment string
	flagForceAdd   bool
)

var Cmd = &cobra.Command{
	Use:   "update-commitment",
	Short: "update commitment",
	Run:   run,
}

func init() {
	Cmd.Flags().StringVarP(&flagDatadir, "datadir", "d", "/var/flow/data/protocol", "directory to the badger dababase")
	_ = Cmd.MarkPersistentFlagRequired("datadir")

	Cmd.Flags().StringVar(&flagBlockID, "block-id", "", "block id")
	_ = Cmd.MarkPersistentFlagRequired("block-id")

	Cmd.Flags().StringVar(&flagCommitment, "commitment", "", "commitment")
	_ = Cmd.MarkPersistentFlagRequired("commitment")

	Cmd.Flags().BoolVar(&flagForceAdd, "force", false, "force adding even if it doesn't exist")
}

func run(*cobra.Command, []string) {
	err := updateCommitment(flagDatadir, flagBlockID, flagCommitment, flagForceAdd)
	if err != nil {
		fmt.Printf("fatal: %v\n", err)
	}
}

func updateCommitment(datadir, blockIDStr, commitStr string, force bool) error {
	log.Info().Msgf("updating commitment for block %s, commitment %s, force %t", blockIDStr, commitStr, force)
	// validate blockID
	blockID, err := flow.HexStringToIdentifier(blockIDStr)
	if err != nil {
		return fmt.Errorf("invalid block id: %v", err)
	}

	stateCommitmentBytes, err := hex.DecodeString(commitStr)
	if err != nil {
		return fmt.Errorf("cannot get decode the state commitment: %v", err)
	}

	commit, err := flow.ToStateCommitment(stateCommitmentBytes)
	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			return fmt.Errorf("invalid state commitment length: %v", err)
		}

		log.Warn().Msgf("commitment not found for block %s", blockIDStr)

		if !force {
			return fmt.Errorf("commitment not found and force flag not set")
		}
	}

	commits, db, err := createStorages(datadir)
	if err != nil {
		return fmt.Errorf("could not create storages: %v", err)
	}
	defer db.Close()

	commitToRemove, err := commits.ByBlockID(blockID)
	if err != nil {
		return fmt.Errorf("could not get commit by block id: %v", err)
	}

	if commitToRemove == commit {
		return fmt.Errorf("commitment to be updated is identical to the current one")
	}

	log.Info().Msgf("found commitment to be removed: %x", commitToRemove)

	writeBatch := storagebadger.NewBatch(db)
	err = commits.BatchRemoveByBlockID(blockID, writeBatch)
	if err != nil {
		return fmt.Errorf("could not batch remove commit by block id: %v", err)
	}

	err = writeBatch.Flush()
	if err != nil {
		return fmt.Errorf("could not flush write batch: %v", err)
	}

	log.Info().Msgf("commitment removed for block %s", blockIDStr)

	log.Info().Msgf("storing new commitment: %x", commit)

	writeBatch = storagebadger.NewBatch(db)
	err = commits.BatchStore(blockID, commit, writeBatch)
	if err != nil {
		return fmt.Errorf("could not store commit: %v", err)
	}
	err = writeBatch.Flush()
	if err != nil {
		return fmt.Errorf("could not flush write batch: %v", err)
	}

	log.Info().Msgf("commitment successfully stored for block %s", blockIDStr)

	return nil
}

func createStorages(dir string) (storage.Commits, *badger.DB, error) {
	db := common.InitStorage(dir)
	if db == nil {
		return nil, nil, fmt.Errorf("could not initialize db")
	}

	storages := common.InitStorages(db)
	return storages.Commits, db, nil
}
