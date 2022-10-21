package automate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
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

var VALUES_HEADER string = "branch: fake-branch\n# Commit must be a string\ncommit: \"123456\"\n\ndefaults: {}\n"

var DEFAULT_ACCESS_IMAGE string = "gcr.io/flow-container-registry/access:v0.27.6"
var DEFAULT_COLLECTION_IMAGE string = "gcr.io/flow-container-registry/collection:v0.27.6"
var DEFAULT_CONSENSUS_IMAGE string = "gcr.io/flow-container-registry/consensus:v0.27.6"
var DEFAULT_EXECUTION_IMAGE string = "gcr.io/flow-container-registry/execution:v0.27.6"
var DEFAULT_VERIFICATION_IMAGE string = "gcr.io/flow-container-registry/verification:v0.27.6"

// Loads node info json file and returns unmarshaled struct and node counts
func loadNodeJsonData(nodeInfoPath string) (map[string]Node, map[string]int) {
	jsonFile, err := os.Open(nodeInfoPath)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Successfully Opened %s", nodeInfoPath)
	defer jsonFile.Close()

	byteValue, _ := ioutil.ReadAll(jsonFile)

	var nodes []Node
	json.Unmarshal(byteValue, &nodes)

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

func textReader(path string) string {
	file, err := os.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}

	return string(file)
}

func writeYamlToFile(filepath string, yamlData []byte) {
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
	template, err := template.New("todos").Parse(textReader(templatePath))
	if err != nil {
		log.Fatal(err)
	}

	return template
}

func loadYamlStructs() {
	templateFolder := "struct_templates/"
	nodesData, nodeConfig := loadNodeJsonData(DEFAULT_NODE_INFO_PATH)
	resources := textReader(templateFolder + RESOURCES_TEMPLATE)
	nodeTypeResources := unmarshalToStruct(resources, &Defaults{}).(*Defaults)

	accessTemplate := createTemplate(templateFolder + ACCESS_TEMPLATE)
	var accessNodeMap = make(map[string]NodeDetails)
	for i := 1; i <= nodeConfig["access"]; i++ {
		name := fmt.Sprint("access", i)
		replacementData := ReplacementData{NodeID: nodesData[name].NodeID, ImageTag: DEFAULT_ACCESS_IMAGE}

		accessNodeMap[name] = *structNodeReplacement(accessTemplate, replacementData)
	}
	var accessNodes = Access{Defaults: *nodeTypeResources, Nodes: accessNodeMap}

	collectionTemplate := createTemplate(templateFolder + COLLECTION_TEMPLATE)
	var collectionNodeMap = make(map[string]NodeDetails)
	for i := 1; i <= nodeConfig["collection"]; i++ {
		name := fmt.Sprint("collection", i)
		replacementData := ReplacementData{NodeID: nodesData[name].NodeID, ImageTag: DEFAULT_COLLECTION_IMAGE}

		collectionNodeMap[name] = *structNodeReplacement(collectionTemplate, replacementData)
	}
	var collectionNodes = Collection{Defaults: *nodeTypeResources, Nodes: collectionNodeMap}

	consensusTemplate := createTemplate(templateFolder + CONSENSUS_TEMPLATE)
	var consensusNodeMap = make(map[string]NodeDetails)
	for i := 1; i <= nodeConfig["consensus"]; i++ {
		name := fmt.Sprint("consensus", i)
		replacementData := ReplacementData{NodeID: nodesData[name].NodeID, ImageTag: DEFAULT_CONSENSUS_IMAGE}

		consensusNodeMap[name] = *structNodeReplacement(consensusTemplate, replacementData)
	}
	var consensusNodes = Consensus{Defaults: *nodeTypeResources, Nodes: consensusNodeMap}

	executionTemplate := createTemplate(templateFolder + EXECUTION_TEMPLATE)
	var executionNodeMap = make(map[string]NodeDetails)
	for i := 1; i <= nodeConfig["execution"]; i++ {
		name := fmt.Sprint("execution", i)
		replacementData := ReplacementData{NodeID: nodesData[name].NodeID, ImageTag: DEFAULT_EXECUTION_IMAGE}

		executionNodeMap[name] = *structNodeReplacement(executionTemplate, replacementData)
	}
	var executionNodes = Execution{Defaults: *nodeTypeResources, Nodes: executionNodeMap}

	verificationTemplate := createTemplate(templateFolder + VERIFICATION_TEMPLATE)
	var verificationNodeMap = make(map[string]NodeDetails)
	for i := 1; i <= nodeConfig["verification"]; i++ {
		name := fmt.Sprint("verification", i)
		replacementData := ReplacementData{NodeID: nodesData[name].NodeID, ImageTag: DEFAULT_VERIFICATION_IMAGE}

		verificationNodeMap[name] = *structNodeReplacement(verificationTemplate, replacementData)
	}
	var verificationNodes = Verification{Defaults: *nodeTypeResources, Nodes: verificationNodeMap}

	values := unmarshalToStruct(VALUES_HEADER, &Values{}).(*Values)
	values.Access = accessNodes
	values.Collection = collectionNodes
	values.Consensus = consensusNodes
	values.Execution = executionNodes
	values.Verification = verificationNodes

	marshalToYaml(values)
}

func unmarshalToStruct(source string, target interface{}) interface{} {
	yaml.Unmarshal([]byte(source), target)
	return target
}

func marshalToYaml(source interface{}) {
	output, _ := yaml.Marshal(source)
	writeYamlToFile("values.yml", output)
}

func structNodeReplacement(nodeTemplate *template.Template, replacementData ReplacementData) *NodeDetails {
	nodeString := replaceTemplateData(nodeTemplate, replacementData)
	nodeStruct := unmarshalToStruct(nodeString, &NodeDetails{}).(*NodeDetails)
	nodeStruct.Image = replacementData.ImageTag
	nodeStruct.NodeID = replacementData.NodeID

	return nodeStruct
}
