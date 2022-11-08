package main

import (
	"flag"
	"github.com/onflow/flow-go/integration/benchnet2/automate/level1"
	"os"
)

// sample usage:
// go run cmd/level1/bootstrap.go  --data "./testdata/level1/data/root-protocol-state-snapshot1.json"
func main() {
	dataFlag := flag.String("data", "", "Path to bootstrap JSON data.")
	flag.Parse()

	if *dataFlag == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}

	bootstrap := level1.NewBootstrap(*dataFlag)
	bootstrap.GenTemplateData(true)
}
