package abi_test

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/magiconair/properties/assert"
	"github.com/nsf/jsondiff"
	"github.com/stretchr/testify/require"

	languageAbi "github.com/dapperlabs/flow-go/language/abi"
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

	for _, assetName := range languageAbi.AssetNames() {

		if strings.HasSuffix(assetName, ".cdc") {

			abiAssetName := assetName + ".abi.json"
			abiAsset, _ := languageAbi.Asset(abiAssetName)

			if abiAsset != nil {

				t.Run(assetName, func(t *testing.T) {

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
