package automate

import (
	"bytes"
	"fmt"
	"testing"
	"text/template"

	"github.com/stretchr/testify/require"
)

type Replacement struct {
	NodeID   string
	ImageTag string
}

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

func TestSubString(t *testing.T) {
	fmt.Println("Starting templates test")
	var replacements Replacement
	replacements.NodeID = "abc123"
	replacements.ImageTag = "v0.27-1234"
	original := textReader("templates/test_templates/templates_test.yml")
	var doc bytes.Buffer

	template_string, err := template.New("todos").Parse(original)
	if err != nil {
		panic(err)
	}
	err = template_string.Execute(&doc, replacements)
	if err != nil {
		panic(err)
	}
	s := doc.String()
	fmt.Println(s)
}
