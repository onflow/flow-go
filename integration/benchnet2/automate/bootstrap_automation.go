package automate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"regexp"

	"github.com/go-yaml/yaml"
)

// Node struct which to unmarshal the node-info into
type Node struct {
	Role          string `json:"Role"`
	Address       string `json:"Address"`
	NodeID        string `json:"NodeID"`
	Weight        int    `json:"Weight"`
	NetworkPubKey string `json:"NetworkPubKey"`
	StakingPubKey string `json:"StakingPubKey"`
}

// ReplacementNodeData struct which contains all data replacements for templates
type ReplacementNodeData struct {
	NodeID   string
	ImageTag string
}

type ReplacementValues struct {
	Branch       string
	Commit       string
	Access       map[string]string
	Collection   map[string]string
	Consensus    map[string]string
	Execution    map[string]string
	Verification map[string]string
}

var DEFAULT_NODE_INFO_PATH = "../bootstrap/public-root-information/node-infos.pub.json"

var ACCESS_TEMPLATE string = "access_template.yml"
var COLLECTION_TEMPLATE string = "collection_template.yml"
var CONSENSUS_TEMPLATE string = "consensus_template.yml"
var EXECUTION_TEMPLATE string = "execution_template.yml"
var VERIFICATION_TEMPLATE string = "verification_template.yml"
var RESOURCES_TEMPLATE string = "resources_template.yml"
var TEMPLATE_PATH string = "templates/"

var DEFAULT_ACCESS_IMAGE string = "gcr.io/flow-container-registry/access:v0.27.6"
var DEFAULT_COLLECTION_IMAGE string = "gcr.io/flow-container-registry/collection:v0.27.6"
var DEFAULT_CONSENSUS_IMAGE string = "gcr.io/flow-container-registry/consensus:v0.27.6"
var DEFAULT_EXECUTION_IMAGE string = "gcr.io/flow-container-registry/execution:v0.27.6"
var DEFAULT_VERIFICATION_IMAGE string = "gcr.io/flow-container-registry/verification:v0.27.6"

// Generates a values file based on a given json and templates
func GenerateValuesYaml(inputJsonFilePath string, templatePath string, outputYamlFilePath string, branch string, commit string) {
	GenerateValuesYamlWithImages(inputJsonFilePath, templatePath, outputYamlFilePath, branch, commit, DEFAULT_ACCESS_IMAGE, DEFAULT_COLLECTION_IMAGE, DEFAULT_CONSENSUS_IMAGE, DEFAULT_EXECUTION_IMAGE, DEFAULT_VERIFICATION_IMAGE)
}

// Generates a values file based on a given json and templates, with addtion of selecting image version for nodes
func GenerateValuesYamlWithImages(inputJsonFilePath string, templatePath string, outputYamlFilePath string, branch string, commit string,
	accessImage string, collectionImage string, consensusImage string, executionImage string, verificationImage string) error {
	templateFolder := TEMPLATE_PATH
	if templatePath != "" {
		templateFolder = templatePath
	}
	nodeInfoJson := DEFAULT_NODE_INFO_PATH
	if inputJsonFilePath != "" {
		nodeInfoJson = inputJsonFilePath
	}
	nodesData, nodeConfig, err := loadNodeJsonData(nodeInfoJson)
	if err != nil {
		return err
	}

	values, err := buildValuesStruct(templateFolder, nodesData, nodeConfig, branch, commit, accessImage, collectionImage, consensusImage, executionImage, verificationImage)
	if err != nil {
		return err
	}

	return marshalToYaml(values, outputYamlFilePath)
}

// Loads node info json file and returns unmarshaled struct and node counts
func loadNodeJsonData(nodeInfoPath string) (map[string]Node, map[string]int, error) {
	jsonFileBytes, err := textReader(nodeInfoPath)
	if err != nil {
		return nil, nil, err
	}

	var nodes []Node
	err = json.Unmarshal(jsonFileBytes, &nodes)
	if err != nil {
		return nil, nil, err
	}

	var nodeConfig = make(map[string]int)
	nodeConfig["access"] = 0
	nodeConfig["collection"] = 0
	nodeConfig["consensus"] = 0
	nodeConfig["execution"] = 0
	nodeConfig["verification"] = 0

	nodeMap := map[string]Node{}
	nodeNameRe := regexp.MustCompile(`\w{6,}\d{1,3}`)
	nodeTypeRe := regexp.MustCompile(`\D{6,}`)
	for _, node := range nodes {
		name := nodeNameRe.FindStringSubmatch(node.Address)
		nodeType := nodeTypeRe.FindStringSubmatch(node.Address)
		nodeMap[name[0]] = node
		nodeConfig[nodeType[0]]++
	}

	return nodeMap, nodeConfig, err
}

// Builds and returns values yaml struct
func buildValuesStruct(templateFolder string, nodesData map[string]Node, nodeConfig map[string]int, branch string, commit string,
	accessImage string, collectionImage string, consensusImage string, executionImage string, verificationImage string) (*Values, error) {
	resources, err := textReader(templateFolder + RESOURCES_TEMPLATE)
	if err != nil {
		return nil, err
	}
	resourcesInterface, err := unmarshalToStruct(resources, &Defaults{})
	if err != nil {
		return nil, err
	}
	nodeTypeResources := resourcesInterface.(*Defaults)

	accessTemplatePath := templateFolder + ACCESS_TEMPLATE
	accessNodeMap, err := nodeStruct("access", accessTemplatePath, accessImage, nodesData, nodeConfig)
	if err != nil {
		return nil, err
	}
	var accessNodes = NodesDefs{Defaults: *nodeTypeResources, Nodes: accessNodeMap}

	collectionTemplatePath := templateFolder + COLLECTION_TEMPLATE
	collectionNodeMap, err := nodeStruct("collection", collectionTemplatePath, collectionImage, nodesData, nodeConfig)
	if err != nil {
		return nil, err
	}
	var collectionNodes = NodesDefs{Defaults: *nodeTypeResources, Nodes: collectionNodeMap}

	consensusTemplatePath := templateFolder + CONSENSUS_TEMPLATE
	consensusNodeMap, err := nodeStruct("consensus", consensusTemplatePath, consensusImage, nodesData, nodeConfig)
	if err != nil {
		return nil, err
	}
	var consensusNodes = NodesDefs{Defaults: *nodeTypeResources, Nodes: consensusNodeMap}

	executionTemplatePath := templateFolder + EXECUTION_TEMPLATE
	executionNodeMap, err := nodeStruct("execution", executionTemplatePath, executionImage, nodesData, nodeConfig)
	if err != nil {
		return nil, err
	}
	var executionNodes = NodesDefs{Defaults: *nodeTypeResources, Nodes: executionNodeMap}

	verificationTemplatePath := templateFolder + VERIFICATION_TEMPLATE
	verificationNodeMap, err := nodeStruct("verification", verificationTemplatePath, verificationImage, nodesData, nodeConfig)
	var verificationNodes = NodesDefs{Defaults: *nodeTypeResources, Nodes: verificationNodeMap}

	return &Values{Branch: branch, Commit: commit, Defaults: EmptyStruct{}, Access: accessNodes, Collection: collectionNodes,
		Consensus: consensusNodes, Execution: executionNodes, Verification: verificationNodes}, err
}

func nodeStruct(nodeType string, nodeTemplatePath string, imageTag string, nodesData map[string]Node, nodeConfig map[string]int) (map[string]NodeDetails, error) {
	nodeTemplate, err := createTemplate(nodeTemplatePath)
	if err != nil {
		return nil, err
	}
	var nodeMap = make(map[string]NodeDetails)

	for i := 1; i <= nodeConfig[nodeType]; i++ {
		name := fmt.Sprint(nodeType, i)
		replacementData := ReplacementNodeData{NodeID: nodesData[name].NodeID, ImageTag: imageTag}

		nodeString, err := replaceTemplateData(nodeTemplate, replacementData)
		if err != nil {
			return nil, err
		}
		nodeInterface, err := unmarshalToStruct([]byte(nodeString), &NodeDetails{})
		if err != nil {
			return nil, err
		}
		nodeStruct := nodeInterface.(*NodeDetails)
		nodeStruct.Image = replacementData.ImageTag
		nodeStruct.NodeID = replacementData.NodeID

		nodeMap[name] = *nodeStruct
	}

	return nodeMap, err
}

func textReader(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func writeYamlBytesToFile(filepath string, yamlData []byte) error {
	file, err := os.OpenFile(filepath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	_, err = file.Write(yamlData)
	if err != nil {
		return err
	}

	return file.Close()
}

func replaceTemplateData(template *template.Template, data interface{}) (string, error) {
	var doc bytes.Buffer
	err := template.Execute(&doc, data)
	return doc.String(), err
}

func createTemplate(templatePath string) (*template.Template, error) {
	templateBytes, err := textReader(templatePath)
	if err != nil {
		return nil, err
	}
	return template.New("todos").Parse(string(templateBytes))
}

func unmarshalToStruct(source []byte, target interface{}) (interface{}, error) {
	err := yaml.Unmarshal(source, target)

	return target, err
}

func marshalToYaml(source interface{}, outputFilePath string) error {
	yamlData, err := yaml.Marshal(source)
	if err != nil {
		return err
	}
	return writeYamlBytesToFile(outputFilePath, yamlData)
}

func iterateTemplates() {
	folder := "test_templates/"
	testTemp, _ := createTemplate(folder + "template_test.yml")
	testmap := make(map[string]string)
	teststring := make(map[string]ReplacementNodeData)

	testmap["one"] = "Map1"
	testmap["two"] = "Map2"

	teststring["one"] = ReplacementNodeData{NodeID: "oneID", ImageTag: "oneImage"}
	teststring["two"] = ReplacementNodeData{NodeID: "twoID", ImageTag: "twoImage"}

	testslice := []string{"1234", "abce"}

	testData := testingStruct{Branch: "replacement branch", Testmap: testmap, Teststring: teststring, Testslice: testslice}

	var doc bytes.Buffer
	err := testTemp.Execute(&doc, testData)

	fmt.Println(err)
	fmt.Println(doc.String())
}

func GenerateValues(inputJsonFilePath string, templatePath string, outputYamlFilePath string, branch string, commit string,
	accessImage string, collectionImage string, consensusImage string, executionImage string, verificationImage string) error {
	templateFolder := "test_templates/"
	if templatePath != "" {
		templateFolder = templatePath
	}
	nodeInfoJson := DEFAULT_NODE_INFO_PATH
	if inputJsonFilePath != "" {
		nodeInfoJson = inputJsonFilePath
	}

	nodesData, nodeConfig, err := loadNodeJsonData(nodeInfoJson)
	if err != nil {
		return err
	}

	valuesData, err := buildValues(templateFolder, nodesData, nodeConfig, "branch", "commit", accessImage, collectionImage, consensusImage, executionImage, verificationImage)
	if err != nil {
		return err
	}

	valuesTemplate, err := createTemplate(templateFolder + "values_template.yml")
	if err != nil {
		return err
	}
	values, err := replaceTemplateData(valuesTemplate, valuesData)
	if err != nil {
		return err
	}

	return writeYamlBytesToFile("values.yml", []byte(values))
}

func buildValues(templateFolder string, nodesData map[string]Node, nodesConfig map[string]int, branch string, commit string,
	accessImage string, collectionImage string, consensusImage string, executionImage string, verificationImage string) (ReplacementValues, error) {
	accessTemplatePath := templateFolder + ACCESS_TEMPLATE
	accessNodes, err := buildNodes("access", accessTemplatePath, accessImage, nodesData, nodesConfig)
	if err != nil {
		return ReplacementValues{}, err
	}

	collectionTemplatePath := templateFolder + COLLECTION_TEMPLATE
	collectionNodes, err := buildNodes("collection", collectionTemplatePath, collectionImage, nodesData, nodesConfig)
	if err != nil {
		return ReplacementValues{}, err
	}

	consensusTemplatePath := templateFolder + CONSENSUS_TEMPLATE
	consensusNodes, err := buildNodes("consensus", consensusTemplatePath, consensusImage, nodesData, nodesConfig)
	if err != nil {
		return ReplacementValues{}, err
	}

	executionTemplatePath := templateFolder + EXECUTION_TEMPLATE
	executionNodes, err := buildNodes("execution", executionTemplatePath, executionImage, nodesData, nodesConfig)
	if err != nil {
		return ReplacementValues{}, err
	}

	verificationTemplatePath := templateFolder + VERIFICATION_TEMPLATE
	verificationNodes, err := buildNodes("verification", verificationTemplatePath, verificationImage, nodesData, nodesConfig)

	return ReplacementValues{Branch: branch, Commit: commit, Access: accessNodes, Collection: collectionNodes, Consensus: consensusNodes, Execution: executionNodes, Verification: verificationNodes}, err
}

func buildNodes(nodeType string, nodeTemplatePath string, imageTag string, nodesData map[string]Node, nodeConfig map[string]int) (map[string]string, error) {
	nodeTemplate, err := createTemplate(nodeTemplatePath)
	if err != nil {
		return nil, err
	}
	var nodeMap = make(map[string]string)

	for i := 1; i <= nodeConfig[nodeType]; i++ {
		name := fmt.Sprint(nodeType, i)
		key := name + ":"
		replacementData := ReplacementNodeData{NodeID: nodesData[name].NodeID, ImageTag: imageTag}

		nodeString, err := replaceTemplateData(nodeTemplate, replacementData)
		if err != nil {
			return nil, err
		}
		nodeMap[key] = nodeString
	}

	return nodeMap, err
}

type testingStruct struct {
	Branch     string
	Testmap    map[string]string
	Teststring map[string]ReplacementNodeData
	Testslice  []string
}

// # test1: {{range.testmap}}{{$index}}{{.}}
// # test2: {{range.teststring}}{{if $index == "one"}}{{.NodeID}}
