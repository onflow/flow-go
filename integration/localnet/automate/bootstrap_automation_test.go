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

	GenerateValuesYaml(nodeConfig)
	textReader("values.yml")
}

func TestGenerateTestTemplates(t *testing.T) {
	fmt.Printf("Starting tests")
	var nodeConfig = make(map[string]int)
	nodeConfig["access"] = 2
	nodeConfig["collection"] = 6
	nodeConfig["consensus"] = 3
	nodeConfig["execution"] = 2
	nodeConfig["verification"] = 1

	generateValuesYaml(nodeConfig, "templates/test_templates/")
	textReader("values.yml")
}

func TestReplaceString(t *testing.T) {
	original := "This contains A_STRING that needs to be replaced"
	expected := "This contains a proper string that needs to be replaced"
	replacement := replaceStrings(original, "A_STRING", "a proper string")

	require.Equal(t, expected, replacement, "Mismatching strings")
}

func TestLoadString(t *testing.T) {
	actual := textReader("bootstrap_test.txt")
	expected := "Test string 123"

	require.Equal(t, expected, actual, "Mismatching strings")
}
