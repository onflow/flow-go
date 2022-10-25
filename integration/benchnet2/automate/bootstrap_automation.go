package automate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
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

// ReplacementData struct which contains all data replacements for templates
type ReplacementData struct {
	NodeID   string
	ImageTag string
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

func GenerateValuesYaml(inputJsonFilePath string, templatePath string, outputYamlFilePath string) {
	templateFolder := TEMPLATE_PATH
	if templatePath != "" {
		templateFolder = templatePath
	}
	nodeInfoJson := DEFAULT_NODE_INFO_PATH
	if inputJsonFilePath != "" {
		nodeInfoJson = inputJsonFilePath
	}
	nodesData, nodeConfig := loadNodeJsonData(nodeInfoJson)

	values := buildValuesStruct(templateFolder, nodesData, nodeConfig)

	marshalToYaml(values, outputYamlFilePath)
}

// Loads node info json file and returns unmarshaled struct and node counts
func loadNodeJsonData(nodeInfoPath string) (map[string]Node, map[string]int) {
	jsonFileBytes := textReader(nodeInfoPath)

	var nodes []Node
	json.Unmarshal(jsonFileBytes, &nodes)

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

	return nodeMap, nodeConfig
}

func buildValuesStruct(templateFolder string, nodesData map[string]Node, nodeConfig map[string]int) *Values {
	resources := string(textReader(templateFolder + RESOURCES_TEMPLATE))
	nodeTypeResources := unmarshalToStruct(resources, &Defaults{}).(*Defaults)

	accessTemplatePath := templateFolder + ACCESS_TEMPLATE
	accessNodeMap := nodeStruct("access", accessTemplatePath, DEFAULT_ACCESS_IMAGE, nodesData, nodeConfig)
	var accessNodes = NodesDefs{Defaults: *nodeTypeResources, Nodes: accessNodeMap}

	collectionTemplatePath := templateFolder + COLLECTION_TEMPLATE
	collectionNodeMap := nodeStruct("collection", collectionTemplatePath, DEFAULT_COLLECTION_IMAGE, nodesData, nodeConfig)
	var collectionNodes = NodesDefs{Defaults: *nodeTypeResources, Nodes: collectionNodeMap}

	consensusTemplatePath := templateFolder + CONSENSUS_TEMPLATE
	consensusNodeMap := nodeStruct("consensus", consensusTemplatePath, DEFAULT_CONSENSUS_IMAGE, nodesData, nodeConfig)
	var consensusNodes = NodesDefs{Defaults: *nodeTypeResources, Nodes: consensusNodeMap}

	executionTemplatePath := templateFolder + EXECUTION_TEMPLATE
	executionNodeMap := nodeStruct("execution", executionTemplatePath, DEFAULT_EXECUTION_IMAGE, nodesData, nodeConfig)
	var executionNodes = NodesDefs{Defaults: *nodeTypeResources, Nodes: executionNodeMap}

	verificationTemplatePath := templateFolder + VERIFICATION_TEMPLATE
	verificationNodeMap := nodeStruct("verification", verificationTemplatePath, DEFAULT_VERIFICATION_IMAGE, nodesData, nodeConfig)
	var verificationNodes = NodesDefs{Defaults: *nodeTypeResources, Nodes: verificationNodeMap}

	return &Values{Branch: "fake-branch", Commit: "123456", Defaults: EmptyStruct{}, Access: accessNodes, Collection: collectionNodes,
		Consensus: consensusNodes, Execution: executionNodes, Verification: verificationNodes}
}

func nodeStruct(nodeType string, nodeTemplatePath string, imageTag string, nodesData map[string]Node, nodeConfig map[string]int) map[string]NodeDetails {
	nodeTemplate := createTemplate(nodeTemplatePath)
	var nodeMap = make(map[string]NodeDetails)

	for i := 1; i <= nodeConfig[nodeType]; i++ {
		name := fmt.Sprint(nodeType, i)
		replacementData := ReplacementData{NodeID: nodesData[name].NodeID, ImageTag: imageTag}

		nodeString := replaceTemplateData(nodeTemplate, replacementData)
		nodeStruct := unmarshalToStruct(nodeString, &NodeDetails{}).(*NodeDetails)
		nodeStruct.Image = replacementData.ImageTag
		nodeStruct.NodeID = replacementData.NodeID

		nodeMap[name] = *nodeStruct
	}

	return nodeMap
}

func textReader(path string) []byte {
	file, err := os.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}

	return file
}

func writeYamlBytesToFile(filepath string, yamlData []byte) {
	file, err := os.OpenFile(filepath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}

	_, err = file.Write(yamlData)
	if err != nil {
		log.Fatal(err)
	}

	file.Close()
}

func replaceTemplateData(template *template.Template, data ReplacementData) string {
	var doc bytes.Buffer
	err := template.Execute(&doc, data)
	if err != nil {
		log.Fatal(err)
	}
	return doc.String()
}

func createTemplate(templatePath string) *template.Template {
	template, err := template.New("todos").Parse(string(textReader(templatePath)))
	if err != nil {
		log.Fatal(err)
	}

	return template
}

func unmarshalToStruct(source string, target interface{}) interface{} {
	yaml.Unmarshal([]byte(source), target)
	return target
}

func marshalToYaml(source interface{}, outputFilePath string) {
	yamlData, err := yaml.Marshal(source)
	if err != nil {
		log.Fatal(err)
	}
	writeYamlBytesToFile(outputFilePath, yamlData)
}
