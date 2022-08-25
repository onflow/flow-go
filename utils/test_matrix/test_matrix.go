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
	// list all packages

	// iterate over the packages to run as separate CI jobs
	for i, packageName := range os.Args[1:] {
		fmt.Printf("Package %d is %s\n", i+1, packageName)
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
		for i, v := range allPackages {
			fmt.Println("allPackages[", i, "]=", v)
		}
		//fmt.Println("allPackagesStr=", allPackagesStr)
	}

	// generate JSON output that will be read in by CI matrix
}
