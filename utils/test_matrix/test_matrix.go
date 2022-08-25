package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Generates a list of packages to test that will be passed to GitHub Actions
func main() {
	fmt.Println("*** Test Matrix Generator ***")

	if len(os.Args) == 1 {
		fmt.Println("must have at least 1 package listed")
		return
	}

	allFlowPackages := listAllFlowPackages()

	//listTargetPackages(allFlowPackages)

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
			if strings.HasPrefix(allPackage, "github.com/onflow/flow-go/"+targetPackagePrefix) {
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

	restPackages := listRestPackages(allFlowPackages, seenPackages)

	fmt.Println("restPackages length: ", string(len(restPackages)))

	fmt.Println("finished generating package list")
	// generate JSON output that will be read in by CI matrix
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
	cmd := exec.Command("go", "list", "./...")
	outBytes, err := cmd.Output()
	if err != nil {
		panic("error during go list: " + err.Error())
	}

	if cmd.ProcessState.ExitCode() != 0 {
		panic("process exit code not 0: " + string(cmd.ProcessState.ExitCode()))
	}

	allPackagesStr := string(outBytes)
	allPackages := strings.Fields(allPackagesStr)

	return allPackages
}
