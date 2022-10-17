package automate

import (
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateTestTemplates(t *testing.T) {
	fmt.Printf("Starting tests")
	outputFilePath := "test_values.yml"
	expectedValues := textReader("templates/test_templates/expected_values.yml")

	GenerateValuesYaml("test_files/sample-infos.pub.json", "test_files/", outputFilePath)
	actualValues := textReader(outputFilePath)

	require.Equal(t, expectedValues, actualValues)

	// cleanup
	deleteFile(outputFilePath)
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

func TestCreateWriteReadYaml(t *testing.T) {
	filepath := "testYaml.yml"
	testString := "Test String 123@"
	file := createFile(filepath)
	yamlWriter(file, "Test String 123@")

	file.Close()

	actualString := textReader(filepath)
	require.Equal(t, actualString, testString)
	deleteFile(filepath)
}

func deleteFile(filepath string) {
	err := os.Remove(filepath)
	if err != nil {
		log.Fatal(err)
	}
}
