package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/onflow/flow-go/integration/benchnet2/automate"
)

// sample test run
// go run cmd/main.go --data "./testdata/data/test1.json" --template "./testdata/templates/test1.yml" --file=true
func main() {
	dataFlag := flag.String("data", "", "Path to JSON data.")
	templateFlag := flag.String("template", "", "Path to template file.")
	fileOutputFlag := flag.Bool("file", false, "Whether to output to a file")
	flag.Parse()

	if *dataFlag == "" || *templateFlag == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}

	template := automate.NewTemplate(*dataFlag, *templateFlag)
	actualOutput := template.Apply(*fileOutputFlag)
	fmt.Println("output=", actualOutput)
}
