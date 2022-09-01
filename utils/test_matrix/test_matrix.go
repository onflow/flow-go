package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"golang.org/x/tools/go/packages"
)

const flowPackagePrefix = "github.com/onflow/flow-go/"

// testMatrix represents a single GitHub Actions test matrix combination that consists of a name and a list of flow-go packages associated with that name.
type testMatrix struct {
	Name     string `json:"name"`
	Packages string `json:"packages"`
}

// Generates a list of packages to test that will be passed to GitHub Actions
func main() {
	if len(os.Args) == 1 {
		fmt.Fprintln(os.Stderr, "must have at least 1 package listed")
		return
	}

	allFlowPackages := listAllFlowPackages()

	targetPackages, seenPackages := listTargetPackages(os.Args[1:], allFlowPackages)

	restPackages := listRestPackages(allFlowPackages, seenPackages)

	// generate JSON output that will be read in by CI matrix
	testMatrix := generateTestMatrix(targetPackages, restPackages)
	testMatrixBytes, err := json.Marshal(testMatrix)
	if err != nil {
		panic(err)
	}

	fmt.Println(
		"::set-output name=matrix::" + string(testMatrixBytes),
	)
}

func generateTestMatrix(targetPackages map[string][]string, restPackages []string) []testMatrix {

	var testMatrices []testMatrix

	for names := range targetPackages {
		targetTestMatrix := testMatrix{
			Name:     names,
			Packages: strings.Join(targetPackages[names], " "),
		}
		testMatrices = append(testMatrices, targetTestMatrix)
	}

	// add the "rest" packages after all target packages added
	restTestMatrix := testMatrix{
		Name:     "rest",
		Packages: strings.Join(restPackages, " "),
	}

	testMatrices = append(testMatrices, restTestMatrix)

	return testMatrices
}

// listTargetPackages returns a map-list of target packages to run as separate CI jobs, based on a list of target package prefixes.
// It also returns a list of the "seen" packages that can then be used to extract the remaining packages to run (in a separate CI job).
func listTargetPackages(targetPackagePrefixes []string, allFlowPackages []string) (map[string][]string, map[string]string) {
	targetPackages := make(map[string][]string)

	// Stores list of packages already seen / allocated to other lists. Needed for the last package which will
	// have all the leftover packages that weren't allocated to a separate list (CI job).
	// It's a map, not a list, to make it easier to check if a package was seen or not.
	seenPackages := make(map[string]string)

	// iterate over the target packages to run as separate CI jobs
	for _, targetPackagePrefix := range targetPackagePrefixes {
		var targetPackage []string

		// go through all packages to see which ones to pull out
		for _, allPackage := range allFlowPackages {
			if strings.HasPrefix(allPackage, flowPackagePrefix+targetPackagePrefix) {
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
