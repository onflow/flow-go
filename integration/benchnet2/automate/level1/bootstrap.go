package level1

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

type Bootstrap struct {
	jsonInput string
	nodeData  []NodeData
}

type NodeData struct {
	id   string
	name string
	role string
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

	// examine "Identities" section for list of node data to extract
	identities := dataMap["Identities"].([]interface{})

	for index, identity := range identities {
		fmt.Println("Reading Value for Key :", index)
		fmt.Println("identity: ", identity)
		identityMap := identity.(map[string]interface{})
		nodeID := identityMap["NodeID"]
		fmt.Println("NodeID: ", nodeID)
		role := identityMap["Role"]
		fmt.Println("Role: ", role)
		address := identityMap["Address"]
		fmt.Println("Address: ", address)
	}
	
	return "{}"
}
