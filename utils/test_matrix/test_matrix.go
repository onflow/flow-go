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
const ciDefaultRunner = "ubuntu-latest"

// testMatrix represents a single GitHub Actions test matrix combination that consists of a name and a list of flow-go packages associated with that name.
type testMatrix struct {
	Name     string `json:"name"`
	Packages string `json:"packages"`
	Runner   string `json:"runner"`
}

type targets struct {
	runners  map[string]string
	packages map[string][]string
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

func generateTestMatrix(targetPackages targets, otherPackages []string) []testMatrix {
	var testMatrices []testMatrix

	for packageName := range targetPackages.packages {
		targetTestMatrix := testMatrix{
			Name:     packageName,
			Packages: strings.Join(targetPackages.packages[packageName], " "),
			Runner:   targetPackages.runners[packageName],
		}
		testMatrices = append(testMatrices, targetTestMatrix)
	}

	// add the other packages after all target packages added
	otherTestMatrix := testMatrix{
		Name:     "others",
		Packages: strings.Join(otherPackages, " "),
		Runner:   ciDefaultRunner,
	}

	testMatrices = append(testMatrices, otherTestMatrix)

	return testMatrices
}

// listTargetPackages returns a map-list of target packages to run as separate CI jobs, based on a list of target package prefixes.
// It also returns a list of the "seen" packages that can then be used to extract the remaining packages to run (in a separate CI job).
func listTargetPackages(targetPackagePrefixes []string, allFlowPackages []string) (targets, map[string]string) {
	targetPackages := make(map[string][]string)
	targetRunners := make(map[string]string)

	// Stores list of packages already seen / allocated to other lists. Needed for the last package which will
	// have all the leftover packages that weren't allocated to a separate list (CI job).
	// It's a map, not a list, to make it easier to check if a package was seen or not.
	seenPackages := make(map[string]string)

	// iterate over the target packages to run as separate CI jobs
	for _, targetPackagePrefix := range targetPackagePrefixes {
		var targetPackage []string

		// assume package name specified without runner
		targetPackagePrefixNoRunner := targetPackagePrefix

		// check if specify CI runner to use for the package, otherwise use default
		colonIndex := strings.Index(targetPackagePrefix, ":")
		if colonIndex != -1 {
			targetPackagePrefixNoRunner = targetPackagePrefix[:colonIndex] // strip out runner from package name
			targetRunners[targetPackagePrefixNoRunner] = targetPackagePrefix[colonIndex+1:]
		} else {
			// use default CI runner if didn't specify runner
			targetRunners[targetPackagePrefix] = ciDefaultRunner
		}

		// go through all packages to see which ones to pull out
		for _, allPackage := range allFlowPackages {
			if strings.HasPrefix(allPackage, flowPackagePrefix+targetPackagePrefixNoRunner) {
				// if the package was already seen, don't append it to the list
				// this is to support listing sub-sub packages in a CI job without duplicating those sub-sub packages
				// in the parent package job
				_, seen := seenPackages[allPackage]
				if seen {
					continue
				}
				targetPackage = append(targetPackage, allPackage)
				seenPackages[allPackage] = allPackage
			}
		}
		if len(targetPackage) == 0 {
			panic("no packages exist with prefix " + targetPackagePrefixNoRunner)
		}
		targetPackages[targetPackagePrefixNoRunner] = targetPackage
	}
	return targets{targetRunners, targetPackages}, seenPackages
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
