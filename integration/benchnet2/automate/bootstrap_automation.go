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
)

// User struct which contains a name
// a type and a list of social links
type Node struct {
	Role          string `json:"Role"`
	Address       string `json:"Address"`
	NodeID        string `json:"NodeID"`
	Weight        int    `json:"Weight"`
	NetworkPubKey string `json:"NetworkPubKey"`
	StakingPubKey string `json:"StakingPubKey"`
}

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
var ENV_TEMPLATE string = "env_template.yml"
var TEMPLATE_PATH string = "templates/"

var VALUES_HEADER string = "branch: fake-branch\n# Commit must be a string\ncommit: \"123456\"\n\ndefaults: {}\n"

var DEFAULT_ACCESS_IMAGE string = "gcr.io/flow-container-registry/access:v0.27.6"
var DEFAULT_COLLECTION_IMAGE string = "gcr.io/flow-container-registry/collection:v0.27.6"
var DEFAULT_CONSENSUS_IMAGE string = "gcr.io/flow-container-registry/consensus:v0.27.6"
var DEFAULT_EXECUTION_IMAGE string = "gcr.io/flow-container-registry/execution:v0.27.6"
var DEFAULT_VERIFICATION_IMAGE string = "gcr.io/flow-container-registry/verification:v0.27.6"

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

func yamlWriter(file *os.File, content string) {
	_, err := file.Write([]byte(content))
	if err != nil {
		log.Fatal(err)
	}
}

func createFile(filename string) *os.File {
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}

	return file
}

func GenerateValuesYaml(jsonDataFilePath string, templatePath string, outputFilePath string) {
	var nodesData map[string]Node
	var nodeConfig map[string]int
	var templateFolder string
	if jsonDataFilePath == "" {
		nodesData, nodeConfig = loadNodeJsonData(DEFAULT_NODE_INFO_PATH)
	} else {
		nodesData, nodeConfig = loadNodeJsonData(jsonDataFilePath)
	}

	if templatePath == "" {
		templateFolder = TEMPLATE_PATH
	} else {
		templateFolder = templatePath
	}

	valuesFilePath := "values.yml"
	if outputFilePath != "" {
		valuesFilePath = outputFilePath
	}
	values := createFile(valuesFilePath)
	log.Printf("Values file created at %s", valuesFilePath)

	envTemplate := createTemplate(templateFolder + ENV_TEMPLATE)
	resources := textReader(templateFolder + RESOURCES_TEMPLATE)

	yamlWriter(values, VALUES_HEADER)

	yamlWriter(values, "access:\n")
	yamlWriter(values, resources)

	accessTemplate := createTemplate(templateFolder + ACCESS_TEMPLATE)
	for i := 1; i <= nodeConfig["access"]; i++ {
		name := fmt.Sprint("access", i)
		replacementData := ReplacementData{NodeID: nodesData[name].NodeID, ImageTag: DEFAULT_ACCESS_IMAGE}

		writeNodeData(values, name, envTemplate, accessTemplate, replacementData)
	}

	yamlWriter(values, "collection:\n")
	yamlWriter(values, resources)

	collectionTemplate := createTemplate(templateFolder + COLLECTION_TEMPLATE)
	for i := 1; i <= nodeConfig["collection"]; i++ {
		name := fmt.Sprint("collection", i)
		replacementData := ReplacementData{NodeID: nodesData[name].NodeID, ImageTag: DEFAULT_COLLECTION_IMAGE}

		writeNodeData(values, name, envTemplate, collectionTemplate, replacementData)
	}

	yamlWriter(values, "consensus:\n")
	yamlWriter(values, resources)

	consensusTemplate := createTemplate(templateFolder + CONSENSUS_TEMPLATE)
	for i := 1; i <= nodeConfig["consensus"]; i++ {
		name := fmt.Sprint("consensus", i)
		replacementData := ReplacementData{NodeID: nodesData[name].NodeID, ImageTag: DEFAULT_CONSENSUS_IMAGE}

		writeNodeData(values, name, envTemplate, consensusTemplate, replacementData)
	}

	yamlWriter(values, "execution:\n")
	yamlWriter(values, resources)

	executionTemplate := createTemplate(templateFolder + EXECUTION_TEMPLATE)
	for i := 1; i <= nodeConfig["execution"]; i++ {
		name := fmt.Sprint("execution", i)
		replacementData := ReplacementData{NodeID: nodesData[name].NodeID, ImageTag: DEFAULT_EXECUTION_IMAGE}

		writeNodeData(values, name, envTemplate, executionTemplate, replacementData)
	}

	yamlWriter(values, "verification:\n")
	yamlWriter(values, resources)

	verificationTemplate := createTemplate(templateFolder + VERIFICATION_TEMPLATE)
	for i := 1; i <= nodeConfig["verification"]; i++ {
		name := fmt.Sprint("verification", i)
		replacementData := ReplacementData{NodeID: nodesData[name].NodeID, ImageTag: DEFAULT_VERIFICATION_IMAGE}

		writeNodeData(values, name, envTemplate, verificationTemplate, replacementData)
	}

	log.Printf("Values files successfully written to %s", outputFilePath)
	values.Close()
}

func writeNodeData(file *os.File, name string, envTemplate *template.Template, data *template.Template, replacementData ReplacementData) {
	replacedData := replaceTemplateData(data, replacementData)
	replacedEnv := replaceTemplateData(envTemplate, replacementData)

	yamlWriter(file, "    "+name+":\n")
	yamlWriter(file, replacedData)
	yamlWriter(file, replacedEnv)
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
