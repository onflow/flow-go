package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/onflow/flow-go/integration/benchnet2/automate/level2"
)

// sample usage:
// go run cmd/level2/template.go --data "./testdata/level2/data/values8-verification-nodes-if-loop.json" --template "./testdata/level2/templates/values8-verification-nodes-if-loop.yml" --outPath="./values.yml"
func main() {
	dataFlag := flag.String("data", "", "Path to JSON data.")
	templateFlag := flag.String("template", "", "Path to template file.")
	outputPathFlag := flag.String("outPath", "", "Helm output file full path.")
	flag.Parse()

	if *dataFlag == "" || *templateFlag == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}

	template := level2.NewTemplate(*dataFlag, *templateFlag)
	actualOutput := template.Apply(*outputPathFlag)
	fmt.Println("output=", actualOutput)
}
