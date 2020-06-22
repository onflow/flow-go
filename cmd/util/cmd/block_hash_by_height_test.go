package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/metrics"
	bstorage "github.com/dapperlabs/flow-go/storage/badger"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestBlockHashByHeight(t *testing.T) {
	metr := metrics.NewCacheCollector(flow.Testnet)

	t.Run("AllowUnfinalizedUnsealed", func(t *testing.T) {
		datadir, err := tempDBDir()
		require.NoError(t, err)
		db := initStorage(datadir)
		headers := bstorage.NewHeaders(metr, db)

		h := unittest.BlockHeaderFixture()
		storeAndIndexHeader(t, db, headers, &h)

		err = db.Close()
		require.NoError(t, err)

		// execute command
		cmd := exec.Command("go", "run", "-tags", "relic", utilPath(t), "block-hash-by-height", "--height", fmt.Sprint(h.Height), "--datadir", datadir, "--allow-unfinalized", "--allow-unsealed")

		// capture the subprocess stdout and stderr
		out, err := cmd.CombinedOutput()

		// check that the process did not fail
		assert.NoError(t, err)
		assert.Equal(t, fmt.Sprintf("%v\n", h.ID()), string(out))
	})

	t.Run("RequireFinalizedAllowUnsealed", func(t *testing.T) {
		datadir, err := tempDBDir()
		require.NoError(t, err)

		db := initStorage(datadir)
		headers := bstorage.NewHeaders(metr, db)

		h1 := unittest.BlockHeaderFixture()
		storeAndIndexHeader(t, db, headers, &h1)
		h2 := unittest.BlockHeaderWithParentFixture(&h1)
		storeAndIndexHeader(t, db, headers, &h2)
		h3 := unittest.BlockHeaderWithParentFixture(&h2)
		storeAndIndexHeader(t, db, headers, &h3)
		h4 := unittest.BlockHeaderWithParentFixture(&h3)
		storeAndIndexHeader(t, db, headers, &h4)

		err = db.Close()
		require.NoError(t, err)

		// execute command
		cmd := exec.Command("go", "run", "-tags", "relic", utilPath(t), "block-hash-by-height", "--height", fmt.Sprint(h1.Height), "--datadir", datadir, "--allow-unsealed")

		// capture the subprocess stdout and stderr
		out, err := cmd.CombinedOutput()

		// check that the process did not fail
		assert.NoError(t, err)
		assert.Equal(t, fmt.Sprintf("%v\n", h1.ID()), string(out))
	})

	t.Run("AllowUnfinalizedRequireSealed", func(t *testing.T) {
		datadir, err := tempDBDir()
		require.NoError(t, err)

		db := initStorage(datadir)
		headers := bstorage.NewHeaders(metr, db)
		seals := bstorage.NewSeals(metr, db)

		h := unittest.BlockHeaderFixture()
		storeAndIndexHeader(t, db, headers, &h)
		storeAndIndexSealFor(t, db, seals, &h)

		err = db.Close()
		require.NoError(t, err)

		// execute command
		cmd := exec.Command("go", "run", "-tags", "relic", utilPath(t), "block-hash-by-height", "--height", fmt.Sprint(h.Height), "--datadir", datadir, "--allow-unfinalized")

		// capture the subprocess stdout and stderr
		out, err := cmd.CombinedOutput()

		// check that the process did not fail
		assert.NoError(t, err)
		assert.Equal(t, fmt.Sprintf("%v\n", h.ID()), string(out))
	})
}

func tempDBDir() (string, error) {
	return ioutil.TempDir("", "flow-bootstrap-db")
}

func storeAndIndexHeader(t *testing.T, db *badger.DB, headers *bstorage.Headers, h *flow.Header) {
	err := headers.Store(h)
	require.NoError(t, err)
	err = db.Update(operation.IndexBlockHeight(h.Height, h.ID()))
	require.NoError(t, err)
}

func storeAndIndexSealFor(t *testing.T, db *badger.DB, seals *bstorage.Seals, h *flow.Header) {
	seal := unittest.BlockSealFixture()
	seal.BlockID = h.ID()

	err := seals.Store(seal)
	require.NoError(t, err)
	err = db.Update(operation.IndexBlockSeal(h.ID(), seal.ID()))
	require.NoError(t, err)
}

func utilPath(t *testing.T) string {
	// get path to util cmd
	path, err := os.Getwd()
	require.NoError(t, err)
	path = filepath.Join(path, "..")
	return path
}
