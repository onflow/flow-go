package level1

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const BootstrapPath = "../testdata/level1/data/"
const ExpectedOutputPath = "../testdata/level1/expected"

func TestGenerateBootstrap_DataTable(t *testing.T) {
	testDataMap := map[string]testData{
		"2AN 6LN 3CN 2 EN 1 VN": {
			bootstrapPath:  filepath.Join(BootstrapPath, "root-protocol-state-snapshot1.json"),
			expectedOutput: filepath.Join(ExpectedOutputPath, "template-data-input1.json"),
		},
	}

	for i, testData := range testDataMap {
		t.Run(i, func(t *testing.T) {
			// generate template data file from bootstrap file
			bootstrap := NewBootstrap(testData.bootstrapPath)
			actualNodeData := bootstrap.GenTemplateData(false)

			// load expected template data file
			var expectedNodeData []NodeData
			expectedDataBytes, err := os.ReadFile(testData.expectedOutput)
			require.NoError(t, err)
			expectedDataStr := string(expectedDataBytes)
			expectedDataStr = strings.Trim(expectedDataStr, "\t \n")
			err = json.Unmarshal(expectedDataBytes, &expectedNodeData)
			require.Nil(t, err)

			// check generated template data file is correct by unmarshalling json into structs
			require.Equal(t, len(expectedNodeData), len(actualNodeData))
			require.ElementsMatch(t, expectedNodeData, actualNodeData)
		})
	}
}

type testData struct {
	bootstrapPath  string
	expectedOutput string
}
