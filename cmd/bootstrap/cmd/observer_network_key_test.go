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

	"github.com/onflow/flow-go/crypto"
	model "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestObserverNetworkKeyFileExists(t *testing.T) {
	unittest.RunWithTempDir(t, func(bootDir string) {
		var keyFileExistsRegex = regexp.MustCompile(`^network key already exists`)

		// command flags
		flagOutdir = bootDir

		hook := zeroLoggerHook{logs: &strings.Builder{}}
		log = log.Hook(hook)

		// run keys command to generate all keys and bootstrap files
		observerNetworkKeyRun(nil, nil)
		hook.logs.Reset()

		require.DirExists(t, flagOutdir)

		// make sure file exists
		networkKeyFilePath := filepath.Join(flagOutdir, model.FilenameObserverNetworkKey)
		require.FileExists(t, networkKeyFilePath)

		// read file priv key file before command
		keyDataBefore := readText(networkKeyFilePath)

		// run command with flags
		observerNetworkKeyRun(nil, nil)

		// make sure regex matches
		require.Regexp(t, keyFileExistsRegex, hook.logs.String())

		// read network key file again
		keyDataAfter := readText(networkKeyFilePath)

		// check if key was modified
		assert.Equal(t, keyDataBefore, keyDataAfter)
	})
}

func TestObserverNetworkKeyFileCreated(t *testing.T) {
	unittest.RunWithTempDir(t, func(bootDir string) {
		var keyFileCreatedRegex = `^generated network key` +
			`wrote file %s/network-key`
		regex := regexp.MustCompile(fmt.Sprintf(keyFileCreatedRegex, bootDir))

		// command flags
		flagOutdir = bootDir

		hook := zeroLoggerHook{logs: &strings.Builder{}}
		log = log.Hook(hook)

		networkKeyFilePath := filepath.Join(flagOutdir, model.FilenameObserverNetworkKey)

		// run command with flags
		observerNetworkKeyRun(nil, nil)

		// make sure regex matches
		assert.Regexp(t, regex, hook.logs.String())

		// make sure file exists (regex checks this too)
		require.FileExists(t, networkKeyFilePath)

		// make sure key is valid and the correct type
		keyData := readText(networkKeyFilePath)
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
