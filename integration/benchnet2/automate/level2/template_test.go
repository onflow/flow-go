package level2

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const DataPath = "../testdata/level2/data/"
const TemplatesPath = "../testdata/level2/templates"
const ExpectedOutputPath = "../testdata/level2/expected"

func TestApply_DataTable(t *testing.T) {
	testDataMap := map[string]testData{
		"simple1": {
			templatePath:   filepath.Join(TemplatesPath, "test1.yml"),
			dataPath:       filepath.Join(DataPath, "test1.json"),
			expectedOutput: filepath.Join(ExpectedOutputPath, "test1.yml"),
		},

		"simple2": {
			templatePath:   filepath.Join(TemplatesPath, "access_template.yml"),
			dataPath:       filepath.Join(DataPath, "access_template.json"),
			expectedOutput: filepath.Join(ExpectedOutputPath, "access_template.yml"),
		},

		"values11 - nested template (defaults)": {
			templatePath:   filepath.Join(TemplatesPath, "values11-nested-template-defaults.yml"),
			dataPath:       filepath.Join(DataPath, "values8-verification-nodes-if-loop.json"), // same data file
			expectedOutput: filepath.Join(ExpectedOutputPath, "values1.yml"),
		},
	}

	for i, testData := range testDataMap {
		t.Run(i, func(t *testing.T) {
			// generate template output based on template and data values
			template := NewTemplate(testData.dataPath, testData.templatePath)
			actualTemplateOutputStr := template.Apply("")

			//load expected template output
			expectedTemplateOutputBytes, err := os.ReadFile(testData.expectedOutput)
			require.NoError(t, err)
			expectedTemplateOutputStr := string(expectedTemplateOutputBytes)
			expectedTemplateOutputStr = strings.Trim(expectedTemplateOutputStr, "\t \n")

			// check generated template output is correct
			require.Equal(t, expectedTemplateOutputStr, actualTemplateOutputStr)
		})
	}
}

type testData struct {
	templatePath   string
	dataPath       string
	expectedOutput string
}
