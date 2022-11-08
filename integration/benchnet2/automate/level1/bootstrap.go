package level1

import (
	"encoding/json"
	"log"
	"os"
	"strings"
)

type Bootstrap struct {
	jsonInput string
	nodeData  []NodeData
}

type NodeData struct {
	Id   string `json:"node_id"`
	Name string `json:"name"`
	Role string `json:"group"`
}

func NewBootstrap(jsonInput string) Bootstrap {
	return Bootstrap{
		jsonInput: jsonInput,
	}
}

func (b *Bootstrap) GenTemplateData(outputToFile bool) string {
	// load bootstrap file
	dataBytes, err := os.ReadFile(b.jsonInput)
	if err != nil {
		log.Fatal(err)
	}

	// map any json data map - we can't use arrays here because the bootstrap json data is not an array of objects
	// this avoids the use of structs in case the json changes
	// https://stackoverflow.com/a/38437140/5719544
	var dataMap map[string]interface{}
	if err := json.Unmarshal([]byte(dataBytes), &dataMap); err != nil {
		log.Fatal(err)
	}

	// examine "Identities" section for list of node data to extract and build out node data list
	identities := dataMap["Identities"].([]interface{})
	var nodeDataList []NodeData

	for _, identity := range identities {
		identityMap := identity.(map[string]interface{})
		nodeID := identityMap["NodeID"].(string)
		role := identityMap["Role"].(string)
		address := identityMap["Address"].(string)
		// address will be in format: "verification1.:3569" so we want to extract the name from before the '.'
		name := strings.Split(address, ".")[0]

		nodeDataList = append(nodeDataList, NodeData{
			Id:   nodeID,
			Role: role,
			Name: name,
		})
	}

	templateData, err := json.Marshal(nodeDataList)
	if err != nil {
		log.Fatal(err)
	}

	templateDatStr := string(templateData)

	return string(templateDatStr)
}
