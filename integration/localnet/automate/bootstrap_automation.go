package automate

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v2"
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

var ACCESS_TEMPLATE string = "templates/access_template.yml"
var COLLECTION_TEMPLATE string = "templates/collection_template.yml"
var CONSENSUS_TEMPLATE string = "templates/consensus_template.yml"
var EXECUTION_TEMPLATE string = "templates/execution_template.yml"
var VERIFICATION_TEMPLATE string = "templates/verification_template.yml"
var RESOURCES_TEMPLATE string = "templates/resources_template.yml"
var ENV_TEMPLATE string = "templates/env_template.yml"

func loadNodeJsonData() map[string]Node {
	var node_info_path = "../bootstrap/public-root-information/node-infos.pub.json"

	jsonFile, err := os.Open(node_info_path)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("Successfully Opened node-infos.pub.json")
	defer jsonFile.Close()

	byteValue, _ := ioutil.ReadAll(jsonFile)

	var nodes []Node
	json.Unmarshal(byteValue, &nodes)

	nodeMap := map[string]Node{}
	re := regexp.MustCompile(`\w{6,}\d{1,3}`)
	for _, node := range nodes {
		name := re.FindStringSubmatch(node.Address)
		nodeMap[name[0]] = node
	}

	return nodeMap
}

func replaceStrings(template string, target string, replacement string) string {
	updated := strings.ReplaceAll(template, target, replacement)
	return updated
}

func yamlReader(path string) map[string]string {
	output := map[string]string{}
	yamlFile, err := ioutil.ReadFile("path")
	if err != nil {
		log.Printf("yamlFile.Get err   #%v ", err)
	}
	err = yaml.Unmarshal(yamlFile, output)
	if err != nil {
		log.Fatalf("Unmarshal: %v", err)
	}

	return output
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

func generateValuesYaml(nodeConfig map[string]int) {
	nodesData := loadNodeJsonData()

	values, err := os.OpenFile("values.yaml", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}

	resources := textReader(RESOURCES_TEMPLATE)
	env := textReader(ENV_TEMPLATE)

	yamlWriter(values, "access:\n")
	yamlWriter(values, resources)

	access_data := textReader(ACCESS_TEMPLATE)
	for i := 0; i < nodeConfig["access"]; i++ {
		name := fmt.Sprint("access", i)
		nodeId := nodesData[name].NodeID

		replacedData := replaceStrings(access_data, nodeId, "REPLACE_NODE_ID")

		yamlWriter(values, name)
		yamlWriter(values, env)
		yamlWriter(values, replacedData)
	}

	yamlWriter(values, "collection:\n")
	yamlWriter(values, resources)

	collection_data := textReader(COLLECTION_TEMPLATE)
	for i := 0; i < nodeConfig["collection"]; i++ {
		name := fmt.Sprint("collection", i)
		nodeId := nodesData[name].NodeID

		replacedData := replaceStrings(collection_data, nodeId, "REPLACE_NODE_ID")

		yamlWriter(values, name)
		yamlWriter(values, env)
		yamlWriter(values, replacedData)
	}

	yamlWriter(values, "consensus:\n")
	yamlWriter(values, resources)

	consensus_data := textReader(CONSENSUS_TEMPLATE)
	for i := 0; i < nodeConfig["consensus"]; i++ {
		name := fmt.Sprint("consensus", i)
		nodeId := nodesData[name].NodeID

		replacedData := replaceStrings(consensus_data, nodeId, "REPLACE_NODE_ID")

		yamlWriter(values, name)
		yamlWriter(values, env)
		yamlWriter(values, replacedData)
	}

	yamlWriter(values, "execution:\n")
	yamlWriter(values, resources)

	execution_data := textReader(EXECUTION_TEMPLATE)
	for i := 0; i < nodeConfig["execution"]; i++ {
		name := fmt.Sprint("execution", i)
		nodeId := nodesData[name].NodeID

		replacedData := replaceStrings(execution_data, nodeId, "REPLACE_NODE_ID")

		yamlWriter(values, name)
		yamlWriter(values, env)
		yamlWriter(values, replacedData)
	}

	yamlWriter(values, "verification:\n")
	yamlWriter(values, resources)

	verification_data := textReader(VERIFICATION_TEMPLATE)
	for i := 0; i < nodeConfig["verification"]; i++ {
		name := fmt.Sprint("verification", i)
		nodeId := nodesData[name].NodeID

		replacedData := replaceStrings(verification_data, nodeId, "REPLACE_NODE_ID")

		yamlWriter(values, name)
		yamlWriter(values, env)
		yamlWriter(values, replacedData)
	}

	values.Close()
}
