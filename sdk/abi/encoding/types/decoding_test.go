package types_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	abiExamples "github.com/dapperlabs/flow-go/language/abi"
	"github.com/dapperlabs/flow-go/language/runtime/cmd/abi"
)

func TestExamples(t *testing.T) {

	for _, abiName := range abiExamples.AssetNames() {
		suffix := ".abi.json"
		if strings.HasSuffix(abiName, suffix) {

			cdcName := abiName[:len(abiName)-len(suffix)]

			cdcAsset, _ := abiExamples.Asset(cdcName)

			if cdcAsset != nil {

				t.Run(abiName, func(t *testing.T) {

					abiAsset, err := abiExamples.Asset(abiName)
					require.NoError(t, err)

					typesFromABI, err := abi.GetTypesFromABIJSONBytes(abiAsset)

					assert.NoError(t, err)

					typesFromCadence := abi.GetTypesFromCadenceCode(string(cdcAsset), cdcName)

					assert.Equal(t, typesFromCadence, typesFromABI)
				})
			}
		}
	}
}
