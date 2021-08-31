package unittest

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/utils/io"
)

func RequireFileEmpty(t *testing.T, path string) {
	require.FileExists(t, path)

	fi, err := os.Stat(path)
	require.NoError(t, err)

	size := fi.Size()

	require.Equal(t, int64(0), size)
}

func WriteJSON(path string, data interface{}) error {
	bz, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	err = os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(path, bz, 0644)
	if err != nil {
		return err
	}

	return nil
}


func ReadJSON(path string, target interface{}) error {

	dat, err := io.ReadFile(path)
	if err != nil {
		return err
	}

	err = json.Unmarshal(dat, target)
	if err != nil {
		return err
	}

	return nil
}
