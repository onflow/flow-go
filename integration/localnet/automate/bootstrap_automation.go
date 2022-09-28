package automate

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
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

func generateYamlSections(templatePath string, image string, nodeID string) string {
	//Subs in image and nodeID into given template, returns string
	return templatePath + image + nodeID
}

func generateValuesYaml(nodeConfig map[string]int) {
	nodesData := loadNodeJsonData()
	for i := 0; i < nodeConfig["access"]; i++ {
		name := fmt.Sprint("access", i)
		nodeId := nodesData[name].NodeID

		fmt.Println(name)
		fmt.Println(nodeId)
	}

	for i := 0; i < nodeConfig["collection"]; i++ {
		name := fmt.Sprint("collection", i)
		nodeId := nodesData[name].NodeID

		fmt.Println(name)
		fmt.Println(nodeId)
	}

	for i := 0; i < nodeConfig["consensus"]; i++ {
		name := fmt.Sprint("consensus", i)
		nodeId := nodesData[name].NodeID

		fmt.Println(name)
		fmt.Println(nodeId)
	}

	for i := 0; i < nodeConfig["verification"]; i++ {
		name := fmt.Sprint("verification", i)
		nodeId := nodesData[name].NodeID

		fmt.Println(name)
		fmt.Println(nodeId)
	}
}
