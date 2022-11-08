package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/onflow/flow-go/integration/benchnet2/automate/level2"
)

// sample usage:
// go run cmd/level2/template.go --data "./testdata/level2/data/values8-verification-nodes-if-loop.json" --template "./testdata/level2/templates/values8-verification-nodes-if-loop.yml" --file=true
func main() {
	dataFlag := flag.String("data", "", "Path to JSON data.")
	templateFlag := flag.String("template", "", "Path to template file.")
	fileOutputFlag := flag.Bool("file", false, "Whether to output to a file")
	flag.Parse()

	if *dataFlag == "" || *templateFlag == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}

	template := level2.NewTemplate(*dataFlag, *templateFlag)
	actualOutput := template.Apply(*fileOutputFlag)
	fmt.Println("output=", actualOutput)
}
