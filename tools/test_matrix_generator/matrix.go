package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/pflag"
	"golang.org/x/tools/go/packages"
)

var (
	//go:embed default-test-matrix-config.json
	defaultTestMatrixConfig string

	//go:embed insecure-module-test-matrix-config.json
	insecureModuleTestMatrixConfig string

	//go:embed integration-module-test-matrix-config.json
	integrationModuleTestMatrixConfig string

	matrixConfigFile string
)

const (
	flowPackagePrefix = "github.com/onflow/flow-go/"
	ciMatrixName      = "dynamicMatrix"
	defaultCIRunner   = "blacksmith-4vcpu-ubuntu-2404"
)

// flowGoPackage configuration for a package to be tested.
type flowGoPackage struct {
	// Name the name of the package where test are located.
	Name string `json:"name"`
	// Runner the runner used for the top level github actions job that runs the tests all the tests in the parent package.
	Runner string `json:"runner,omitempty"`
	// Exclude list of packages to exclude from top level parent package test matrix.
	Exclude []string `json:"exclude,omitempty"`
	// Subpackages list of subpackages of the parent package that should be run in their own github actions job.
	Subpackages []*subpackage `json:"subpackages,omitempty"`
}

// subpackage configuration for a subpackage.
type subpackage struct {
	Name   string `json:"name"`
	Runner string `json:"runner,omitempty"`
}

// config the test matrix configuration for a package.
type config struct {
	// PackagesPath director where to load packages from.
	PackagesPath string `json:"packagesPath,omitempty"`
	// IncludeOthers when set to true will put all packages and subpackages of the packages path into a test matrix that will run in a job called others.
	IncludeOthers bool `json:"includeOthers,omitempty"`
	// Packages configurations for all packages that test should be run from.
	Packages []*flowGoPackage `json:"packages"`
}

// testMatrix represents a single GitHub Actions test matrix combination that consists of a name and a list of flow-go packages associated with that name.
type testMatrix struct {
	Name     string `json:"name"`
	Packages string `json:"packages"`
	Runner   string `json:"runner"`
}

// newTestMatrix returns a new testMatrix, if runner is empty "" set the runner to the defaultCIRunner.
func newTestMatrix(name, runner, pkgs string) *testMatrix {
	t := &testMatrix{
		Name:     name,
		Packages: pkgs,
		Runner:   runner,
	}

	if t.Runner == "" {
		t.Runner = defaultCIRunner
	}

	return t
}

// Generates a list of packages to test that will be passed to GitHub Actions
func main() {
	pflag.Parse()

	var configFile string
	switch matrixConfigFile {
	case "insecure":
		configFile = insecureModuleTestMatrixConfig
	case "integration":
		configFile = integrationModuleTestMatrixConfig
	default:
		configFile = defaultTestMatrixConfig
	}

	packageConfig := loadPackagesConfig(configFile)

	testMatrices := buildTestMatrices(packageConfig, listAllFlowPackages)
	printCIString(testMatrices)
}

// printCIString encodes the test matrices as a compact JSON string and writes it to the $GITHUB_OUTPUT file so the
// CI workflow can read it as a step output and make the data available to the test matrix jobs. The JSON is also
// printed to stdout for local debugging.
func printCIString(testMatrices []*testMatrix) {
	// generate JSON output that will be read in by CI matrix
	// can't use json.MarshalIndent because fromJSON() in CI can’t read JSON with any spaces
	b, err := json.Marshal(testMatrices)
	if err != nil {
		panic(fmt.Errorf("failed to marshal test matrices json: %w", err))
	}
	if outputFile := os.Getenv("GITHUB_OUTPUT"); outputFile != "" {
		f, err := os.OpenFile(outputFile, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			panic(fmt.Errorf("failed to open GITHUB_OUTPUT file: %w", err))
		}
		defer f.Close()
		// the compacted JSON is a single line, so the simple name=value format is sufficient (no multiline delimiter needed)
		if _, err := fmt.Fprintf(f, "%s=%s\n", ciMatrixName, b); err != nil {
			panic(fmt.Errorf("failed to write test matrix to GITHUB_OUTPUT: %w", err))
		}
	}
	fmt.Println(string(b))
}

// buildTestMatrices builds the test matrices.
func buildTestMatrices(packageConfig *config, flowPackages func(dir string) []*packages.Package) []*testMatrix {
	testMatrices := make([]*testMatrix, 0)
	seenPaths := make(map[string]struct{})
	seenPath := func(p string) {
		seenPaths[p] = struct{}{}
	}
	seen := func(p string) bool {
		_, seen := seenPaths[p]
		return seen
	}

	for _, topLevelPkg := range packageConfig.Packages {
		allPackages := flowPackages(topLevelPkg.Name)
		// first build test matrix for each of the subpackages and mark all complete paths seen
		subPkgMatrices := processSubpackages(topLevelPkg.Subpackages, allPackages, seenPath)
		testMatrices = append(testMatrices, subPkgMatrices...)
		// now build top level test matrix
		topLevelTestMatrix := processTopLevelPackage(topLevelPkg, allPackages, seenPath, seen)
		testMatrices = append(testMatrices, topLevelTestMatrix)
	}

	// any packages left out of the explicit Packages field will be run together as "others" from the config PackagesPath
	if packageConfig.IncludeOthers {
		allPkgs := flowPackages(packageConfig.PackagesPath)
		if othersTestMatrix := buildOthersTestMatrix(allPkgs, seen); othersTestMatrix != nil {
			testMatrices = append(testMatrices, othersTestMatrix)
		}
	}
	return testMatrices
}

// processSubpackages creates a test matrix for all subpackages provided.
func processSubpackages(subPkgs []*subpackage, allPkgs []*packages.Package, seenPath func(p string)) []*testMatrix {
	testMatrices := make([]*testMatrix, 0)
	for _, subPkg := range subPkgs {
		pkgPath := fullGoPackagePath(subPkg.Name)
		// this is the list of allPackages that used with the go test command
		var testPkgStrBuilder strings.Builder
		for _, p := range allPkgs {
			if strings.HasPrefix(p.PkgPath, pkgPath) {
				testPkgStrBuilder.WriteString(fmt.Sprintf("%s ", p.PkgPath))
				seenPath(p.PkgPath)
			}
		}
		testMatrices = append(testMatrices, newTestMatrix(subPkg.Name, subPkg.Runner, testPkgStrBuilder.String()))
	}
	return testMatrices
}

// processTopLevelPackage creates test matrix for the top level package excluding any packages from the exclude list.
func processTopLevelPackage(pkg *flowGoPackage, allPkgs []*packages.Package, seenPath func(p string), seen func(p string) bool) *testMatrix {
	var topLevelTestPkgStrBuilder strings.Builder
	for _, p := range allPkgs {
		if !seen(p.PkgPath) {
			includePkg := true
			for _, exclude := range pkg.Exclude {
				if strings.HasPrefix(p.PkgPath, fullGoPackagePath(exclude)) {
					includePkg = false
				}
			}

			if includePkg && strings.HasPrefix(p.PkgPath, fullGoPackagePath(pkg.Name)) {
				topLevelTestPkgStrBuilder.WriteString(fmt.Sprintf("%s ", p.PkgPath))
				seenPath(p.PkgPath)
			}
		}
	}
	return newTestMatrix(pkg.Name, pkg.Runner, topLevelTestPkgStrBuilder.String())
}

// buildOthersTestMatrix builds an others test matrix that includes all packages in a path not explicitly set in the packages list of a config.
func buildOthersTestMatrix(allPkgs []*packages.Package, seen func(p string) bool) *testMatrix {
	var othersTestPkgStrBuilder strings.Builder
	for _, otherPkg := range allPkgs {
		if !seen(otherPkg.PkgPath) {
			othersTestPkgStrBuilder.WriteString(fmt.Sprintf("%s ", otherPkg.PkgPath))
		}
	}

	if othersTestPkgStrBuilder.Len() > 0 {
		return newTestMatrix("others", "", othersTestPkgStrBuilder.String())
	}

	return nil
}

func listAllFlowPackages(dir string) []*packages.Package {
	flowPackages, err := packages.Load(&packages.Config{Dir: dir}, "./...")
	if err != nil {
		panic(err)
	}
	return flowPackages
}

func loadPackagesConfig(configFile string) *config {
	var packageConfig config
	buf := bytes.NewBufferString(configFile)
	err := json.NewDecoder(buf).Decode(&packageConfig)
	if err != nil {
		panic(fmt.Errorf("failed to decode package config json %w: %s", err, configFile))
	}
	return &packageConfig
}

func fullGoPackagePath(pkg string) string {
	return fmt.Sprintf("%s%s", flowPackagePrefix, pkg)
}

func init() {
	// Add flags to the FlagSet
	pflag.StringVarP(&matrixConfigFile, "config", "c", "", "the config file used to generate the test matrix")
}
