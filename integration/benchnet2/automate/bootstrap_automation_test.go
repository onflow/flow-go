package automate

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

var TEST_FILES string = "test_files/"

func TestSubString(t *testing.T) {
	expectedMatched := "templates_test:\nreplacement1: 1\nreplacement2: 2"
	expectedUndermatched := "templates_test:\nreplacement1: 1"
	// expectedOvermatched := "templates_test:\nreplacement1: 1\nreplacement2: 2\nreplacement3: {{.ReplaceThree}}"

	matched, _ := createTemplate("test_files/test_matched_template.yml")
	undermatched, _ := createTemplate("test_files/test_undermatched_template.yml")
	// overmatched := createTemplate("test_files/test_overmatched_template.yml")

	replacementData := ReplacementNodeData{NodeID: "1", ImageTag: "2"}

	matchedString, _ := replaceTemplateData(matched, replacementData)
	undermatchedString, _ := replaceTemplateData(undermatched, replacementData)
	// overmatchedString := replaceTemplateData(overmatched, replacementData)

	require.Equal(t, expectedMatched, matchedString)
	require.Equal(t, expectedUndermatched, undermatchedString)
	// require.Equal(t, expectedOvermatched, overmatchedString)
}

func TestReadYaml(t *testing.T) {
	filepath := "test_files/test_read.yml"
	testString := "Test String 123@"

	actualString, err := textReader(filepath)
	require.NoError(t, err)
	require.Equal(t, string(actualString), testString)
}

func TestUnmarshal(t *testing.T) {
	envTemplate, err := textReader(TEST_FILES + ACCESS_TEMPLATE)
	require.NoError(t, err)

	envInterface, err := unmarshalToStruct(envTemplate, &NodeDetails{})
	envStruct := envInterface.(*NodeDetails)
	require.NoError(t, err)
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

	e := writeYamlBytesToFile(filename, []byte(testString))
	if e != nil {
		log.Fatal(e)
	}

	actual, err := textReader(filename)
	require.NoError(t, err)

	require.Equal(t, testString, string(actual))

	deleteFile(filename)
}

func TestMarshalFileWrite(t *testing.T) {
	resource := Resource{CPU: "Intel", Memory: "128GB"}

	expected := "cpu: Intel\nmemory: 128GB\n"
	filename := "test_marshal_write.yml"

	e := marshalToYaml(resource, filename)
	if e != nil {
		log.Fatal(e)
	}

	actual, err := textReader(filename)
	require.NoError(t, err)

	require.Equal(t, expected, string(actual))
	deleteFile(filename)
}

func TestLoadJson(t *testing.T) {
	testJson := TEST_FILES + "sample-infos.pub.json"

	nodesData, nodesConfig, err := loadNodeJsonData(testJson)
	require.NoError(t, err)

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
	nodeData["access1"] = Node{NodeID: "access1_replacement_nodeID"}
	nodeData["collection1"] = Node{NodeID: "collection1_replacement_nodeID"}
	nodeData["consensus1"] = Node{NodeID: "consensus1_replacement_nodeID"}
	nodeData["execution1"] = Node{NodeID: "execution1_replacement_nodeID"}
	nodeData["verification1"] = Node{NodeID: "verification1_replacement_nodeID"}

	nodeConfig := make(map[string]int)
	nodeConfig["access"] = 1
	nodeConfig["collection"] = 1
	nodeConfig["consensus"] = 1
	nodeConfig["execution"] = 1
	nodeConfig["verification"] = 1

	values, err := buildValuesStruct(TEST_FILES, nodeData, nodeConfig, "test-branch", "test-commit-hash", "accessTestImage", "collectionTestImage", "consensusTestImage", "executionTestImage", "verificationTestImage")
	require.NoError(t, err)

	require.Equal(t, "test-branch", values.Branch)
	require.Equal(t, "test-commit-hash", values.Commit)

	require.Equal(t, 1, len(values.Access.Nodes))
	require.Equal(t, "accessTestImage", values.Access.Nodes["access1"].Image)
	require.Equal(t, "collectionTestImage", values.Collection.Nodes["collection1"].Image)
	require.Equal(t, "consensusTestImage", values.Consensus.Nodes["consensus1"].Image)
	require.Equal(t, "executionTestImage", values.Execution.Nodes["execution1"].Image)
	require.Equal(t, "verificationTestImage", values.Verification.Nodes["verification1"].Image)

	require.Equal(t, "access1_replacement_nodeID", values.Access.Nodes["access1"].NodeID)
	require.Equal(t, "collection1_replacement_nodeID", values.Collection.Nodes["collection1"].NodeID)
	require.Equal(t, "consensus1_replacement_nodeID", values.Consensus.Nodes["consensus1"].NodeID)
	require.Equal(t, "execution1_replacement_nodeID", values.Execution.Nodes["execution1"].NodeID)
	require.Equal(t, "verification1_replacement_nodeID", values.Verification.Nodes["verification1"].NodeID)
}

func deleteFile(filepath string) {
	err := os.Remove(filepath)
	if err != nil {
		log.Fatal(err)
	}
}

func TestTemplating(t *testing.T) {
	iterateTemplates()
}

func TestTemplating2(t *testing.T) {
	require.NoError(t, GenerateValues("", "", "values.yml", "test branch", "test_commit", "AccessImage", "CollectionImage", "ConsensusImage", "ExecutionImage", "VerificationImage"))
}

const DataPath = "./testdata/data/"
const TemplatesPath = "./testdata/templates/"
const ExpectedTemplatesPath = "./testdata/expected/"

func TestApply_DataTable(t *testing.T) {
	testDataMap := map[string]testData{
		//"test1": {
		//	templatePath:     filepath.Join(TemplatesPath, "test1.yml"),
		//	dataPath:         filepath.Join(DataPath, "test1.json"),
		//	expectedTemplate: filepath.Join(ExpectedTemplatesPath, "test1.yml"),
		//},
		//
		//"access template": {
		//	templatePath:     filepath.Join(TemplatesPath, "access_template.yml"),
		//	dataPath:         filepath.Join(DataPath, "access_template.json"),
		//	expectedTemplate: filepath.Join(ExpectedTemplatesPath, "access_template.yml"),
		//},
		//
		//"values full - original": {
		//	templatePath:     filepath.Join(TemplatesPath, "values1-original.yml"),
		//	dataPath:         filepath.Join(DataPath, "values1-original.json"),
		//	expectedTemplate: filepath.Join(ExpectedTemplatesPath, "values1.yml"),
		//},
		//
		//"values full - access nodes - separate node ids": {
		//	templatePath:     filepath.Join(TemplatesPath, "values2-access-nodes.yml"),
		//	dataPath:         filepath.Join(DataPath, "values2-access-nodes.json"),
		//	expectedTemplate: filepath.Join(ExpectedTemplatesPath, "values1.yml"),
		//},

		"values full - access nodes - loop": {
			templatePath:     filepath.Join(TemplatesPath, "values3-access-nodes-loop.yml"),
			dataPath:         filepath.Join(DataPath, "values3-access-nodes-loop.json"),
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
