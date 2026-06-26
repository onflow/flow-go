package checkpoint_list_tries

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/complete/wal"
)

var (
	flagCheckpoint string
)

var Cmd = &cobra.Command{
	Use:   "checkpoint-list-tries",
	Short: "Lists tries (root hashes, base64 encoded) of tries stored in a checkpoint",
	Run:   run,
}

func init() {

	Cmd.Flags().StringVar(&flagCheckpoint, "checkpoint", "",
		"checkpoint file to read")
	_ = Cmd.MarkFlagRequired("checkpoint")
}

func run(*cobra.Command, []string) {

	log.Info().Msgf("reading trie root hashes from checkpoint %v", flagCheckpoint)

	hashes, err := readTrieRootHashes(log.Logger, flagCheckpoint)
	if err != nil {
		log.Fatal().Err(err).Msg("error while reading trie root hashes from checkpoint")
	}
	log.Info().Msgf("checkpoint read, total tries: %v", len(hashes))

	for _, h := range hashes {
		fmt.Printf("trie root hash: %s\n", h)
	}
}

// readTrieRootHashes reads only the trie root hashes from the checkpoint file at
// the given path, without materializing the full trie forest. Only the top-trie
// part file (containing the trie root records) is read.
//
// Both V6 and V7 (payloadless) checkpoints are supported; the version is
// determined by the V7 filename suffix ([wal.V7FileSuffix]). The root hashes are
// returned in the order they are stored in the checkpoint.
//
// No error returns are expected during normal operation.
func readTrieRootHashes(logger zerolog.Logger, checkpointFilePath string) ([]ledger.RootHash, error) {
	dir, fileName := filepath.Split(checkpointFilePath)
	if strings.HasSuffix(fileName, wal.V7FileSuffix) {
		return wal.ReadTriesRootHashV7(logger, dir, fileName)
	}
	return wal.ReadTriesRootHash(logger, dir, fileName)
}
