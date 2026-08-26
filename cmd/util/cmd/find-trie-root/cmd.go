package find_trie_root

import (
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"path/filepath"

	prometheusWAL "github.com/onflow/wal/wal"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
)

var (
	flagExecutionStateDir string
	flagRootHash          string
	flagFrom              int
	flagTo                int
	flagBackupDir         string
	flagTrimAsLatestWAL   bool
)

// find trie root hash from the wal files.
// useful for state extraction and rolling back executed height.
// for instance, when extracting state for a target height, it requires the wal files
// has the trie root hash of the target block as the latest few records. If not the case,
// then it is necessary to trim the wal files to the last record with the target trie root hash.
// in order to do that, this command can be used to find the trie root hash in the wal files,
// and copy the wal that contains the trie root hash to a new directory and trim it to
// have the target trie root hash as the last record.
// after that, the new wal file can be used to extract the state for the target height.
var Cmd = &cobra.Command{
	Use:   "find-trie-root",
	Short: "find trie root hash from the wal files",
	Run:   run,
}

func init() {
	Cmd.Flags().StringVar(&flagExecutionStateDir, "execution-state-dir", "/var/flow/data/execution",
		"directory to the execution state")
	_ = Cmd.MarkFlagRequired("execution-state-dir")

	Cmd.Flags().StringVar(&flagRootHash, "root-hash", "",
		"ledger root hash (hex-encoded, 64 characters)")
	_ = Cmd.MarkFlagRequired("root-hash")

	Cmd.Flags().IntVar(&flagFrom, "from", 0, "from segment")
	Cmd.Flags().IntVar(&flagTo, "to", math.MaxInt32, "to segment")

	Cmd.Flags().StringVar(&flagBackupDir, "backup-dir", "",
		"directory for backup wal files. must be not exist or empty folder. required when --trim-as-latest-wal flag is set to true.")

	Cmd.Flags().BoolVar(&flagTrimAsLatestWAL, "trim-as-latest-wal", false,
		"trim the wal file to the last record with the target trie root hash")
}

func run(*cobra.Command, []string) {
	rootHash, err := parseInput(flagRootHash)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot parse input")
	}

	if flagExecutionStateDir == flagBackupDir {
		log.Fatal().Msg("--backup-dir directory cannot be the same as the execution state directory")
	}

	// making sure the backup dir is empty
	empty, err := checkFolderIsEmpty(flagBackupDir)
	if err != nil {
		log.Fatal().Msgf("--backup-dir directory %v must exist and empty", flagBackupDir)
	}

	if !empty {
		log.Fatal().Msgf("--backup-dir directory %v must be empty", flagBackupDir)
	}

	segment, offset, err := common.SearchRootHashBackward(rootHash, flagExecutionStateDir, flagFrom, flagTo)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot find root hash in segments")
	}

	segmentFile := prometheusWAL.SegmentName(flagExecutionStateDir, segment)

	log.Info().Msgf("found root hash in segment %d at offset %d, segment file: %v", segment, offset, segmentFile)

	if !flagTrimAsLatestWAL {
		log.Info().Msg("not trimming WAL. Exiting. to trim the WAL, use --trim-as-latest-wal flag")
		return
	}

	if len(flagBackupDir) == 0 {
		log.Error().Msgf("--backup-dir directory is not provided")
		return
	}

	// create a temporary folder in the backup folder to store the new segment file
	tmpFolder := filepath.Join(flagBackupDir, "flow-last-segment-file")

	log.Info().Msgf("creating temporary folder %v", tmpFolder)

	err = os.Mkdir(tmpFolder, os.ModePerm)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot create temporary folder")
	}

	defer func() {
		log.Info().Msgf("removing temporary folder %v", tmpFolder)
		err := os.RemoveAll(tmpFolder)
		if err != nil {
			log.Error().Err(err).Msg("cannot remove temporary folder")
		}
	}()

	// generate a segment file in the temporary folder with the root hash as its last record
	newSegmentFile, err := common.TrimWALSegmentToHash(flagExecutionStateDir, segment, rootHash, tmpFolder)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot trim WAL segment")
	}

	log.Info().Msgf("successfully created trimmed WAL segment at %v", newSegmentFile)

	// before replacing the last wal file with the newly generated one, backup the rollbacked wals
	// then move the last segment file to the execution state directory
	err = common.BackupAndReplaceWALSegment(segment, flagExecutionStateDir, flagBackupDir, newSegmentFile)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot backup rollbacked WALs")
	}

	log.Info().Msgf("successfully trimmed WAL %v the trie root hash %v as its last record, original wal files are moved to %v",
		segment, rootHash, flagBackupDir)
}

func parseInput(rootHashStr string) (ledger.RootHash, error) {
	rootHashBytes, err := hex.DecodeString(rootHashStr)
	if err != nil {
		return ledger.RootHash(hash.DummyHash), fmt.Errorf("cannot decode root hash: %w", err)
	}
	rootHash, err := ledger.ToRootHash(rootHashBytes)
	if err != nil {
		return ledger.RootHash(hash.DummyHash), fmt.Errorf("invalid root hash: %w", err)
	}
	return rootHash, nil
}

func checkFolderIsEmpty(folderPath string) (bool, error) {
	// Check if the folder exists
	info, err := os.Stat(folderPath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Info().Msgf("folder %v does not exist, creating the folder", folderPath)

			// create the folder if not exist
			err = os.MkdirAll(folderPath, os.ModePerm)
			if err != nil {
				return false, fmt.Errorf("Cannot create the folder.")
			}

			return true, nil
		}
		return false, err
	}

	// Check if the path is a directory
	if !info.IsDir() {
		return false, fmt.Errorf("The path is not a directory.")
	}

	// Check if the folder is empty
	files, err := os.ReadDir(folderPath)
	if err != nil {
		return false, fmt.Errorf("Cannot read the folder.")
	}

	return len(files) == 0, nil
}

