package automate

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateValues(t *testing.T) {
	fmt.Printf("Starting tests")
	var nodeConfig = make(map[string]int)
	nodeConfig["access"] = 2
	nodeConfig["collection"] = 6
	nodeConfig["consensus"] = 3
	nodeConfig["execution"] = 2
	nodeConfig["verification"] = 1

	GenerateValuesYaml(nodeConfig, "", "/templates", "")
	textReader("values.yml")
}

func TestGenerateTestTemplates(t *testing.T) {
	fmt.Printf("Starting tests")
	expectedValues := textReader("templates/test_templates/expected_values.yml")

	var nodeConfig = make(map[string]int)
	nodeConfig["access"] = 2
	nodeConfig["collection"] = 6
	nodeConfig["consensus"] = 3
	nodeConfig["execution"] = 2
	nodeConfig["verification"] = 1

	GenerateValuesYaml(nodeConfig, "templates/test_templates/sample-infos.pub.json", "templates/test_templates", "")
	actualValues := textReader("values.yml")

	require.Equal(t, expectedValues, actualValues)
}

func TestLoadString(t *testing.T) {
	actual := textReader("bootstrap_test.txt")
	expected := "Test string 123"

	require.Equal(t, expected, actual, "Mismatching strings")
}

func TestSubString(t *testing.T) {
	fmt.Println("Starting templates test")

	expectedMatched := "templates_test:\nreplacement1: 1\nreplacement2: 2"
	expectedUndermatched := "templates_test:\nreplacement1: 1"
	// expectedOvermatched := "templates_test:\nreplacement1: 1\nreplacement2: 2\nreplacement3: {{.ReplaceThree}}"

	matched := createTemplate("templates/test_templates/test_matched_template.yml")
	undermatched := createTemplate("templates/test_templates/test_undermatched_template.yml")
	// overmatched := createTemplate("templates/test_templates/test_overmatched_template.yml")

	replacementData := ReplacementData{NodeID: "1", ImageTag: "2"}

	matchedString := replaceTemplateData(matched, replacementData)
	undermatchedString := replaceTemplateData(undermatched, replacementData)
	// overmatchedString := replaceTemplateData(overmatched, replacementData)

	require.Equal(t, expectedMatched, matchedString)
	require.Equal(t, expectedUndermatched, undermatchedString)
	// require.Equal(t, expectedOvermatched, overmatchedString)
}
