package types_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nsf/jsondiff"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/language/runtime/cmd/abi"
)

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func TestExamples(t *testing.T) {

	abiExamplesDir := os.Getenv("FLOW_ABI_EXAMPLES_DIR")

	require.NotEmpty(t, abiExamplesDir, "Set FLOW_ABI_EXAMPLES_DIR env variable to point to directory with Flow ABI example files or use `make test`")

	files, err := ioutil.ReadDir(abiExamplesDir)

	require.NoError(t, err)

	for _, file := range files {

		fullFilepath := filepath.Join(abiExamplesDir, file.Name())

		if strings.HasSuffix(file.Name(), ".abi.json") {

			t.Run(file.Name(), func(t *testing.T) {
				abiBytes, err := ioutil.ReadFile(fullFilepath)

				require.NoError(t, err)

				generatedAbi := abi.GetABIForFile("examples/"+file.Name(), false)

				options := jsondiff.DefaultConsoleOptions()
				diff, s := jsondiff.Compare(generatedAbi, abiBytes, &options)

				assert.Equal(t, diff, jsondiff.FullMatch)

				println(s)
			})
		}
	}
}
