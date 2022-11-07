package automate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const DataPath = "./testdata/data/"
const TemplatesPath = "./testdata/templates/"
const ExpectedTemplatesPath = "./testdata/expected/"

func TestApply_DataTable(t *testing.T) {
	testDataMap := map[string]testData{
		"simple1": {
			templatePath:     filepath.Join(TemplatesPath, "test1.yml"),
			dataPath:         filepath.Join(DataPath, "test1.json"),
			expectedTemplate: filepath.Join(ExpectedTemplatesPath, "test1.yml"),
		},

		"simple2": {
			templatePath:     filepath.Join(TemplatesPath, "access_template.yml"),
			dataPath:         filepath.Join(DataPath, "access_template.json"),
			expectedTemplate: filepath.Join(ExpectedTemplatesPath, "access_template.yml"),
		},

		"values1 - original, empty template": {
			templatePath:     filepath.Join(TemplatesPath, "values1-original.yml"),
			dataPath:         filepath.Join(DataPath, "values1-original.json"),
			expectedTemplate: filepath.Join(ExpectedTemplatesPath, "values1.yml"),
		},

		"values2 - access nodes - separate node ids": {
			templatePath:     filepath.Join(TemplatesPath, "values2-access-nodes.yml"),
			dataPath:         filepath.Join(DataPath, "values2-access-nodes.json"),
			expectedTemplate: filepath.Join(ExpectedTemplatesPath, "values1.yml"),
		},
		//
		//"values3 - access nodes - loop": {
		//	templatePath:     filepath.Join(TemplatesPath, "values3-access-nodes-loop.yml"),
		//	dataPath:         filepath.Join(DataPath, "values3-access-nodes-loop.json"),
		//	expectedTemplate: filepath.Join(ExpectedTemplatesPath, "values1.yml"),
		//},
		//
		//"values4 - access nodes - if loop": {
		//	templatePath:     filepath.Join(TemplatesPath, "values4-access-nodes-if-loop.yml"),
		//	dataPath:         filepath.Join(DataPath, "values4-access-nodes-if-loop.json"),
		//	expectedTemplate: filepath.Join(ExpectedTemplatesPath, "values1.yml"),
		//},
		//
		//"values5 - collection nodes - if loop": {
		//	templatePath:     filepath.Join(TemplatesPath, "values5-collection-nodes-if-loop.yml"),
		//	dataPath:         filepath.Join(DataPath, "values5-collection-nodes-if-loop.json"),
		//	expectedTemplate: filepath.Join(ExpectedTemplatesPath, "values1.yml"),
		//},

		"values6 - consensus nodes - if loop": {
			templatePath:     filepath.Join(TemplatesPath, "values6-consensus-nodes-if-loop.yml"),
			dataPath:         filepath.Join(DataPath, "values6-consensus-nodes-if-loop.json"),
			expectedTemplate: filepath.Join(ExpectedTemplatesPath, "values1.yml"),
		},
	}

	for k, testData := range testDataMap {
		t.Run(k, func(t *testing.T) {
			//load expected template
			expectedTemplateBytes, err := os.ReadFile(testData.expectedTemplate)
			require.NoError(t, err)
			expectedOutputStr := string(expectedTemplateBytes)
			expectedOutputStr = strings.Trim(expectedOutputStr, "\t \n")

			templ := NewTemplate(testData.dataPath, testData.templatePath)
			actualOutput := templ.Apply(false)
			require.Equal(t, expectedOutputStr, actualOutput)
		})
	}
}

type testData struct {
	templatePath     string
	dataPath         string
	expectedTemplate string
}
