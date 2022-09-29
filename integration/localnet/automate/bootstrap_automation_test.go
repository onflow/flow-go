package automate

import (
	"fmt"
	"testing"
)

func TestGeneratedDataAccess(t *testing.T) {
	fmt.Printf("Starting tests")
	var nodeConfig = make(map[string]int)
	nodeConfig["access"] = 2
	nodeConfig["collection"] = 6
	nodeConfig["consensus"] = 3
	nodeConfig["execution"] = 2
	nodeConfig["verification"] = 1

	generateValuesYaml(nodeConfig)
}

func TestReplaceString(t *testing.T) {
	original := "This contains A_STRING that needs to be replaced"
	replacement := replaceStrings(original, "A_STRING", "a proper string")

	fmt.Println("Replacement Strings")
	fmt.Println(replacement)
}
