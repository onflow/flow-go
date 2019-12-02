package abi

import (
	"strings"
	"testing"

	"github.com/magiconair/properties/assert"
	"github.com/nsf/jsondiff"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/language/runtime/cmd/abi"
)

func TestExamples(t *testing.T) {

	for _, assetName := range AssetNames() {
		if strings.HasSuffix(assetName, ".cdc") {

			abiAssetName := assetName + ".abi.json"

			abiAsset, _ := Asset(abiAssetName)

			if abiAsset != nil {

				t.Run(assetName, func(t *testing.T) {

					assetBytes, err := Asset(assetName)
					require.NoError(t, err)

					generatedAbi := abi.GetABIForBytes(assetBytes, false, assetName)

					options := jsondiff.DefaultConsoleOptions()
					diff, s := jsondiff.Compare(generatedAbi, abiAsset, &options)

					assert.Equal(t, diff, jsondiff.FullMatch)

					println(s)
				})
			}
		}
	}
}
