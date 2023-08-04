package cmd

import (
	"encoding/hex"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/crypto"
	"github.com/onflow/flow-go/utils/io"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestObserverNetworkKeyFileExists(t *testing.T) {
	unittest.RunWithTempDir(t, func(bootDir string) {
		// command flags
		flagOutputFile = filepath.Join(bootDir, "test-network-key")

		keyFileExistsRegex := regexp.MustCompile(fmt.Sprintf(`^%s already exists`, flagOutputFile))

		hook := zeroLoggerHook{logs: &strings.Builder{}}
		log = log.Hook(hook)

		// run keys command to generate all keys and bootstrap files
		observerNetworkKeyRun(nil, nil)
		hook.logs.Reset()

		// make sure file exists
		require.FileExists(t, flagOutputFile)

		// read file priv key file before command
		keyDataBefore, err := io.ReadFile(flagOutputFile)
		require.NoError(t, err)

		// run command with flags
		observerNetworkKeyRun(nil, nil)

		// make sure regex matches
		require.Regexp(t, keyFileExistsRegex, hook.logs.String())

		// read network key file again
		keyDataAfter, err := io.ReadFile(flagOutputFile)
		require.NoError(t, err)

		// check if key was modified
		assert.Equal(t, keyDataBefore, keyDataAfter)
	})
}

func TestObserverNetworkKeyFileCreated(t *testing.T) {
	unittest.RunWithTempDir(t, func(bootDir string) {
		// command flags
		flagOutputFile = filepath.Join(bootDir, "test-network-key")

		keyFileCreatedRegex := `^generated network key` +
			`wrote file %s`
		regex := regexp.MustCompile(fmt.Sprintf(keyFileCreatedRegex, flagOutputFile))

		hook := zeroLoggerHook{logs: &strings.Builder{}}
		log = log.Hook(hook)

		// run command with flags
		observerNetworkKeyRun(nil, nil)

		// make sure regex matches
		assert.Regexp(t, regex, hook.logs.String())

		// make sure file exists (regex checks this too)
		require.FileExists(t, flagOutputFile)

		// make sure key is valid and the correct type
		keyData, err := io.ReadFile(flagOutputFile)
		require.NoError(t, err)

		validateNetworkKey(t, keyData)
	})
}

func validateNetworkKey(t *testing.T, keyData []byte) {
	keyBytes := make([]byte, hex.DecodedLen(len(keyData)))
	_, err := hex.Decode(keyBytes, keyData)
	assert.NoError(t, err)

	_, err = crypto.DecodePrivateKey(crypto.ECDSASecp256k1, keyBytes)
	assert.NoError(t, err)
}
