package automate

import (
	"log"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

var TEST_FILES string = "test_files/"

func TestSubString(t *testing.T) {
	expectedMatched := "templates_test:\nreplacement1: 1\nreplacement2: 2"
	expectedUndermatched := "templates_test:\nreplacement1: 1"
	// expectedOvermatched := "templates_test:\nreplacement1: 1\nreplacement2: 2\nreplacement3: {{.ReplaceThree}}"

	matched := createTemplate("test_files/test_matched_template.yml")
	undermatched := createTemplate("test_files/test_undermatched_template.yml")
	// overmatched := createTemplate("test_files/test_overmatched_template.yml")

	replacementData := ReplacementData{NodeID: "1", ImageTag: "2"}

	matchedString := replaceTemplateData(matched, replacementData)
	undermatchedString := replaceTemplateData(undermatched, replacementData)
	// overmatchedString := replaceTemplateData(overmatched, replacementData)

	require.Equal(t, expectedMatched, matchedString)
	require.Equal(t, expectedUndermatched, undermatchedString)
	// require.Equal(t, expectedOvermatched, overmatchedString)
}

func TestReadYaml(t *testing.T) {
	filepath := "test_files/test_read.yml"
	testString := "Test String 123@"

	actualString := string(textReader(filepath))
	require.Equal(t, actualString, testString)
}

func TestUnmarshal(t *testing.T) {
	envTemplate := string(textReader(TEST_FILES + ACCESS_TEMPLATE))

	envStruct := unmarshalToStruct(envTemplate, &NodeDetails{}).(*NodeDetails)
	require.Equal(t, 2, len(envStruct.Args))
	require.Equal(t, "--bootstrapdir=/bootstrap", envStruct.Args[0])
	require.Equal(t, "--access", envStruct.Args[1])
	require.Equal(t, 2, len(envStruct.Env))
	require.Equal(t, "access name", envStruct.Env[0].Name)
	require.Equal(t, "access {{.NodeID}}", envStruct.Env[0].Value)
	require.Equal(t, "access numbers", envStruct.Env[1].Name)
	require.Equal(t, "123", envStruct.Env[1].Value)
}

func TestByteFileWrite(t *testing.T) {
	testString := "yaml: some data\nline2: 123\n"
	filename := "test_file_write.yml"

	writeYamlBytesToFile(filename, []byte(testString))

	actual := string(textReader(filename))

	require.Equal(t, testString, actual)

	deleteFile(filename)
}

func TestMarshalFileWrite(t *testing.T) {
	resource := Resource{CPU: "Intel", Memory: "128GB"}

	expected := "cpu: Intel\nmemory: 128GB\n"
	filename := "test_marshal_write.yml"

	marshalToYaml(resource, filename)

	actual := string(textReader(filename))

	require.Equal(t, expected, actual)
	deleteFile(filename)
}

func TestLoadJson(t *testing.T) {
	testJson := TEST_FILES + "sample-infos.pub.json"

	nodesData, nodesConfig := loadNodeJsonData(testJson)

	require.Equal(t, 1, nodesConfig["access"])
	require.Equal(t, 2, nodesConfig["collection"])
	require.Equal(t, 2, nodesConfig["consensus"])
	require.Equal(t, 1, nodesConfig["execution"])
	require.Equal(t, 1, nodesConfig["verification"])

	require.Equal(t, "access1_nodeID", nodesData["access1"].NodeID)
	require.Equal(t, "collection1_nodeID", nodesData["collection1"].NodeID)
	require.Equal(t, "collection2_nodeID", nodesData["collection2"].NodeID)
	require.Equal(t, "consensus1_nodeID", nodesData["consensus1"].NodeID)
	require.Equal(t, "consensus2_nodeID", nodesData["consensus2"].NodeID)
	require.Equal(t, "execution1_nodeID", nodesData["execution1"].NodeID)
	require.Equal(t, "verification1_nodeID", nodesData["verification1"].NodeID)
}

func TestBuildStruct(t *testing.T) {
	nodeData := make(map[string]Node)
	nodeData["access1"] = Node{NodeID: "access1_nodeID"}
}

func deleteFile(filepath string) {
	err := os.Remove(filepath)
	if err != nil {
		log.Fatal(err)
	}
}
