package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/bootstrap"
	ioutils "github.com/onflow/flow-go/utils/io"
	"github.com/onflow/flow-go/utils/unittest"
)

// Test that attempting to generate a db encryption key is a no-op if a
// key file already exists.
func TestDBEncryptionKeyFileExists(t *testing.T) {
	unittest.RunWithTempDir(t, func(bootDir string) {

		var keyFileExistsRegex = regexp.MustCompile(`^DB encryption key already exists`)

		flagOutdir = bootDir
		flagRole = "verification"
		flagAddress = "189.123.123.42:3869"

		// generate all bootstrapping files
		keyCmdRun(nil, nil)
		require.DirExists(t, filepath.Join(flagOutdir, bootstrap.DirnamePublicBootstrap))
		require.DirExists(t, filepath.Join(flagOutdir, bootstrap.DirPrivateRoot))

		nodeIDPath := filepath.Join(flagOutdir, bootstrap.PathNodeID)
		require.FileExists(t, nodeIDPath)
		b, err := ioutils.ReadFile(nodeIDPath)
		require.NoError(t, err)
		nodeID := strings.TrimSpace(string(b))

		// make sure file exists
		encryptionKeyPath := filepath.Join(flagOutdir, fmt.Sprintf(bootstrap.PathSecretsEncryptionKey, nodeID))
		require.FileExists(t, encryptionKeyPath)

		// read the file
		keyFileBefore, err := ioutils.ReadFile(encryptionKeyPath)
		require.NoError(t, err)

		// create a hooked logger
		var hook unittest.LoggerHook
		log, hook = unittest.HookedLogger()

		// run the encryption key gen tool
		dbEncryptionKeyRun(nil, nil)

		// ensure regex matches
		require.Regexp(t, keyFileExistsRegex, hook.Logs())

		// ensure the existing file is unmodified
		keyFileAfter, err := ioutils.ReadFile(encryptionKeyPath)
		require.NoError(t, err)
		assert.Equal(t, keyFileBefore, keyFileAfter)
	})
}

// Test that we can generate a key file if none exists.
func TestDBEncryptionKeyFileCreated(t *testing.T) {
	unittest.RunWithTempDir(t, func(bootDir string) {

		var keyFileCreatedRegex = `^generated db encryption key` +
			`wrote file ` + bootDir + `/private-root-information/private-node-info_\S+/secretsdb-key`

		flagOutdir = bootDir
		flagRole = "verification"
		flagAddress = "189.123.123.42:3869"

		// generate all bootstrapping files
		keyCmdRun(nil, nil)
		require.DirExists(t, filepath.Join(flagOutdir, bootstrap.DirnamePublicBootstrap))
		require.DirExists(t, filepath.Join(flagOutdir, bootstrap.DirPrivateRoot))

		nodeIDPath := filepath.Join(flagOutdir, bootstrap.PathNodeID)
		require.FileExists(t, nodeIDPath)
		b, err := ioutils.ReadFile(nodeIDPath)
		require.NoError(t, err)
		nodeID := strings.TrimSpace(string(b))

		// delete db encryption key file
		dbEncryptionKeyPath := filepath.Join(flagOutdir, fmt.Sprintf(bootstrap.PathSecretsEncryptionKey, nodeID))
		err = os.Remove(dbEncryptionKeyPath)
		require.NoError(t, err)

		// confirm file was removed
		require.NoFileExists(t, dbEncryptionKeyPath)

		// create a hooked logger
		var hook unittest.LoggerHook
		log, hook = unittest.HookedLogger()

		// run the encryption key gen tool
		dbEncryptionKeyRun(nil, nil)

		// ensure regex matches
		require.Regexp(t, keyFileCreatedRegex, hook.Logs())

		// make sure file exists (regex checks this too)
		require.FileExists(t, dbEncryptionKeyPath)
	})
}
