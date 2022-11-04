package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"golang.org/x/tools/go/packages"
)

const flowPackagePrefix = "github.com/onflow/flow-go/"
const ciMatrixName = "dynamicMatrix"

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

	otherPackages := listOtherPackages(allFlowPackages, seenPackages)

	testMatrix := generateTestMatrix(targetPackages, otherPackages)

	// generate JSON output that will be read in by CI matrix
	// can't use json.MarshalIndent because fromJSON() in CI can’t read JSON with any spaces
	testMatrixBytes, err := json.Marshal(testMatrix)
	if err != nil {
		panic(err)
	}

	// this string will be read by CI to generate groups of tests to run in separate CI jobs
	testMatrixStr := "::set-output name=" + ciMatrixName + "::" + string(testMatrixBytes)

	// very important to add newline character at the end of the compacted JSON - otherwise fromJSON() in CI will throw unmarshalling error
	fmt.Println(testMatrixStr)
}

func generateTestMatrix(targetPackages map[string][]string, otherPackages []string) []testMatrix {

	var testMatrices []testMatrix

	for names := range targetPackages {
		targetTestMatrix := testMatrix{
			Name:     names,
			Packages: strings.Join(targetPackages[names], " "),
		}
		testMatrices = append(testMatrices, targetTestMatrix)
	}

	// add the other packages after all target packages added
	otherTestMatrix := testMatrix{
		Name:     "others",
		Packages: strings.Join(otherPackages, " "),
	}

	testMatrices = append(testMatrices, otherTestMatrix)

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

// listOtherPackages compiles the remaining packages that don't match any of the target packages.
func listOtherPackages(allFlowPackages []string, seenPackages map[string]string) []string {
	var otherPackages []string

	for _, allFlowPackage := range allFlowPackages {
		_, seen := seenPackages[allFlowPackage]
		if !seen {
			otherPackages = append(otherPackages, allFlowPackage)
		}
	}

	if len(otherPackages) == 0 {
		panic("other packages list can't be 0")
	}
	return otherPackages
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
