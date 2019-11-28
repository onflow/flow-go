package abi

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/magiconair/properties/assert"
	"github.com/nsf/jsondiff"
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

	files, err := ioutil.ReadDir("examples/")

	require.NoError(t, err)

	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".cdc") {
			abiFile := "examples/" + file.Name() + ".abi.json"

			if fileExists(abiFile) {

				t.Run(file.Name(), func(t *testing.T) {
					abiBytes, err := ioutil.ReadFile(abiFile)

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
}
