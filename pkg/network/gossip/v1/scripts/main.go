package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"

	"go/format"
)

func genUsage() {
	fmt.Printf("Usage: %s [OPTIONS] file.pb.go\n", os.Args[0])
	flag.PrintDefaults()
}

var (
	save = flag.Bool("w", false, "saves generated code into file")
)

func main() {
	flag.Usage = genUsage
	flag.Parse()

	if len(flag.Args()) < 1 {
		flag.Usage()
		os.Exit(255)
	}

	filepath := flag.Args()[0]

	generatedCode, err := generateCodeFromFile(filepath)
	if err != nil {
		log.Fatalf("could not generate code from given file: %v", err)
	}

	if *save {
		newfilename := genNewFileName(filepath)
		file, err := os.Create(newfilename)

		if err != nil {
			log.Fatalf("could not open file %v for saving: %v", newfilename, err)
		}

		_, err = file.WriteString(generatedCode)
		if err != nil {
			log.Fatalf("could not write generated code to file %v: %v", newfilename, err)
		}
		os.Exit(0)
	}

	fmt.Print(generatedCode)
	os.Exit(0)

}

// fileInfo contains variables needed to fill the registry Template
type fileInfo struct {
	Package    string
	Registries []registryInfo
}

type registryInfo struct {
	InterfaceLong  string
	InterfaceShort string
	Methods        []method
}

// generateCodeFromFile generates a gossip registry code from the given generated
// protobuf file containing a grpc server
func generateCodeFromFile(filepath string) (string, error) {
	var genRegistry = ""
	var err error = nil

	file, err := os.Open(filepath)
	if err != nil {
		return genRegistry, fmt.Errorf("could not open file %v: %v", filepath, err)
	}

	defer func() {
		err = file.Close()
	}()

	infos, err := parseCode(file)
	if err != nil {
		return "", fmt.Errorf("could not parse file %v: %v", filepath, err)
	}

	genRegistry, err = generateRegistry(infos)
	if err != nil {
		return "", fmt.Errorf("could not generate registry: %v", err)
	}

	return genRegistry, err
}

// generateRegistry executes the registry template with data from the
// registry Info and returns formatted go code
func generateRegistry(infos *fileInfo) (string, error) {

	var codeBuffer bytes.Buffer
	codeBuffWriter := bufio.NewWriter(&codeBuffer)

	// Execute template
	if err := registryTemplate.Execute(codeBuffWriter, infos); err != nil {
		return "", fmt.Errorf("could not execute registry template: %v", err)
	}

	// Flush writer's contents
	if err := codeBuffWriter.Flush(); err != nil {
		return "", fmt.Errorf("could not flush the registry's code buffer writer : %v", err)
	}

	//  Format generated go code
	//	equivalent of running gofmt on generated code
	formatedRegistry, err := format.Source(codeBuffer.Bytes())
	if err != nil {
		return "", fmt.Errorf("could not format generated registry code: %v", err)
	}

	return string(formatedRegistry), nil
}

// method holds needed information about method signature in interface
// definition
type method struct {
	Name      string
	ParamType string
}

// newMethod initalizes a method
func newMethod() method {
	return method{
		Name:      "",
		ParamType: "",
	}
}
