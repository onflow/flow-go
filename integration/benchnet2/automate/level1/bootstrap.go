package level1

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
)

type Bootstrap struct {
	// Path to bootstrap JSON data
	protocolJsonFilePath string
}

type NodeData struct {
	Id             string `json:"node_id"`
	Name           string `json:"name"`
	Role           string `json:"role"`
	DockerTag      string `json:"docker_tag"`
	DockerRegistry string `json:"docker_registry"`
}

func NewBootstrap(protocolJsonFilePath string) Bootstrap {
	return Bootstrap{
		protocolJsonFilePath: protocolJsonFilePath,
	}
}

func (b *Bootstrap) GenTemplateData(outputToFile bool, dockerTag string, dockerRegistry string) []NodeData {
	// load bootstrap file
	dataBytes, err := os.ReadFile(b.protocolJsonFilePath)
	if err != nil {
		log.Fatal(err)
	}

	// map any json data map - we can't use arrays here because the bootstrap json data is not an array of objects
	// this avoids the use of structs in case the json changes
	// https://stackoverflow.com/a/38437140/5719544
	var dataMap map[string]interface{}
	err = json.Unmarshal(dataBytes, &dataMap)
	if err != nil {
		log.Fatal(err)
	}

	// examine "Identities" section for list of node data to extract and build out node data list
	identities := dataMap["Epochs"].(map[string]any)["Current"].(map[string]any)["InitialIdentities"].([]any)
	var nodeDataList []NodeData

	for _, identity := range identities {
		identityMap := identity.(map[string]interface{})
		nodeID := identityMap["NodeID"].(string)
		role := identityMap["Role"].(string)
		address := identityMap["Address"].(string)
		// address will be in format: "verification1.:3569" so we want to extract the name from before the '.'
		name := strings.Split(address, ".")[0]

		nodeDataList = append(nodeDataList, NodeData{
			Id:             nodeID,
			Role:           role,
			Name:           name,
			DockerTag:      dockerTag,
			DockerRegistry: dockerRegistry,
		})
	}

	if outputToFile {
		nodeDataBytes, err := json.MarshalIndent(nodeDataList, "", "    ")
		if err != nil {
			log.Fatal(err)
		}

		// create the file
		f, err := os.Create("template-data.json")
		if err != nil {
			fmt.Println(err)
		}
		defer f.Close()

		_, e := f.WriteString(string(nodeDataBytes))
		if e != nil {
			log.Fatal(e)
		}
	}

	return nodeDataList
}
