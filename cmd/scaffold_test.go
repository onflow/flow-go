package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/cmd/bootstrap/utils"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestLoadSecretsEncryptionKey checks that the key file is read correctly if it exists
// and returns the expected sentinel error if it does not exist.
func TestLoadSecretsEncryptionKey(t *testing.T) {
	myID := unittest.IdentifierFixture()

	unittest.RunWithTempDir(t, func(dir string) {
		path := filepath.Join(dir, fmt.Sprintf(bootstrap.PathSecretsEncryptionKey, myID))

		t.Run("should return ErrNotExist if file doesn't exist", func(t *testing.T) {
			require.NoFileExists(t, path)
			_, err := loadSecretsEncryptionKey(dir, myID)
			assert.Error(t, err)
			assert.True(t, errors.Is(err, os.ErrNotExist))
		})

		t.Run("should return key and no error if file exists", func(t *testing.T) {
			err := os.MkdirAll(filepath.Join(dir, bootstrap.DirPrivateRoot, fmt.Sprintf("private-node-info_%v", myID)), 0700)
			require.NoError(t, err)
			key, err := utils.GenerateSecretsDBEncryptionKey()
			require.NoError(t, err)
			err = ioutil.WriteFile(path, key, 0700)
			require.NoError(t, err)

			data, err := loadSecretsEncryptionKey(dir, myID)
			assert.NoError(t, err)
			assert.Equal(t, key, data)
		})
	})
}
