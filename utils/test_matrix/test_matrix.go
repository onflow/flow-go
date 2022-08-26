package main

import (
	"fmt"
	"golang.org/x/tools/go/packages"
	"os"
	"strings"
)

const flowPackagePrefix = "github.com/onflow/flow-go/"

// Generates a list of packages to test that will be passed to GitHub Actions
func main() {
	fmt.Println("*** Test Matrix Generator ***")

	if len(os.Args) == 1 {
		fmt.Println("must have at least 1 package listed")
		return
	}

	allFlowPackages := listAllFlowPackages()

	targetPackages, seenPackages := listTargetPackages(allFlowPackages)

	println(fmt.Sprint("targetPackages lengh=", len(targetPackages)))

	restPackages := listRestPackages(allFlowPackages, seenPackages)

	fmt.Sprint("restPackages length: ", len(restPackages))

	fmt.Println("finished generating package list")
	// generate JSON output that will be read in by CI matrix
}

func listTargetPackages(allFlowPackages []string) (map[string][]string, map[string]string) {
	targetPackages := make(map[string][]string)

	// Stores list of packages already seen / allocated to other lists. Needed for the last package which will
	// have all the leftover packages that weren't allocated to a separate list (CI job).
	// It's a map, not a list, to make it easier to check if a package was seen or not.
	seenPackages := make(map[string]string)

	// iterate over the target packages to run as separate CI jobs
	for i, targetPackagePrefix := range os.Args[1:] {
		fmt.Printf("Package prefix %d is %s\n", i+1, targetPackagePrefix)
		var targetPackage []string

		// go through all packages to see which ones to pull out
		for _, allPackage := range allFlowPackages {
			if strings.HasPrefix(allPackage, flowPackagePrefix+targetPackagePrefix) {
				fmt.Println("found package match: ", allPackage)
				targetPackage = append(targetPackage, allPackage)
				seenPackages[allPackage] = allPackage
			}
		}
		if len(targetPackage) == 0 {
			panic("no packages exist with prefix " + targetPackagePrefix)
		}
		targetPackages[targetPackagePrefix] = targetPackage
	}
	return targetPackages, seenPackages
}

func listRestPackages(allFlowPackages []string, seenPackages map[string]string) []string {
	// compile "the rest" packages
	var restPackages []string

	for _, allFlowPackage := range allFlowPackages {
		_, seen := seenPackages[allFlowPackage]
		if !seen {
			restPackages = append(restPackages, allFlowPackage)
		}
	}

	if len(restPackages) == 0 {
		panic("rest package list can't be 0")
	}
	return restPackages
}

func listAllFlowPackages() []string {
	flowPackages, err := packages.Load(&packages.Config{}, "./...")

	if err != nil {
		panic(err)
	}
	var flowPackagesStr []string
	for _, p := range flowPackages {
		flowPackagesStr = append(flowPackagesStr, p.PkgPath)
	}
	return flowPackagesStr
}
