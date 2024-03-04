package main

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/packages"
)

// TestLoadPackagesConfig ensure packages config json loads as expected.
func TestLoadPackagesConfig(t *testing.T) {
	configFile := `{"packagesPath": ".", "includeOthers": true, "packages": [{"name": "testPackage"}]}`
	config := loadPackagesConfig(configFile)
	if config.PackagesPath != "." || !config.IncludeOthers || len(config.Packages) != 1 {
		t.Errorf("loadPackagesConfig failed for valid input")
	}

	invalidConfigFile := "invalidJSON"
	defer func() {
		if recover() == nil {
			t.Errorf("loadPackagesConfig did not panic for invalid JSON input")
		}
	}()
	loadPackagesConfig(invalidConfigFile)
}

// TestBuildMatrices ensures test matrices are built from config json as expected.
func TestBuildMatrices(t *testing.T) {
	t.Run("top level package only default runner", func(t *testing.T) {
		name := "counter"
		configFile := fmt.Sprintf(`{"packagesPath": ".", "includeOthers": true, "packages": [{"name": "%s"}]}`, name)
		allPackges := goPackageFixture("counter/count", "counter/print/int", "counter/log")
		cfg := loadPackagesConfig(configFile)
		matrices := buildTestMatrices(cfg, func(dir string) []*packages.Package {
			return allPackges
		})
		require.Equal(t, name, matrices[0].Name)
		require.Equal(t, defaultCIRunner, matrices[0].Runner)
		require.Equal(t, fmt.Sprintf("%s %s %s ", allPackges[0].PkgPath, allPackges[1].PkgPath, allPackges[2].PkgPath), matrices[0].Packages)
		fmt.Println(matrices[0].Name, matrices[0].Runner, matrices[0].Packages)
	})
	t.Run("top level package only override runner", func(t *testing.T) {
		name := "counter"
		runner := "buildjet-4vcpu-ubuntu-2204"
		configFile := fmt.Sprintf(`{"packagesPath": ".", "packages": [{"name": "%s", "runner":  "%s"}]}`, name, runner)
		allPackges := goPackageFixture("counter/count", "counter/print/int", "counter/log")
		cfg := loadPackagesConfig(configFile)
		matrices := buildTestMatrices(cfg, func(dir string) []*packages.Package {
			return allPackges
		})
		require.Equal(t, name, matrices[0].Name)
		require.Equal(t, runner, matrices[0].Runner)
		require.Equal(t, fmt.Sprintf("%s %s %s ", allPackges[0].PkgPath, allPackges[1].PkgPath, allPackges[2].PkgPath), matrices[0].Packages)
		fmt.Println(matrices[0].Name, matrices[0].Runner, matrices[0].Packages)
	})
	t.Run("top level package with sub packages include others", func(t *testing.T) {
		topLevelPkgName := "network"
		subPkg1 := "network/p2p/node"
		subPkg2 := "module/chunks"
		subPkg3 := "crypto/hash"
		subPkg4 := "model/bootstrap"
		subPkg1Runner := "buildjet-4vcpu-ubuntu-2204"
		configFile := fmt.Sprintf(`
			{"packagesPath": ".", "includeOthers": true, "packages": [{"name": "%s", "subpackages": [{"name": "%s", "runner": "%s"}, {"name": "%s"}, {"name": "%s"}, {"name": "%s"}]}]}`,
			topLevelPkgName, subPkg1, subPkg1Runner, subPkg2, subPkg3, subPkg4)
		allPackges := goPackageFixture(
			"network",
			"network/alsp",
			"network/cache",
			"network/channels",
			"network/p2p/node",
			"network/p2p/node/internal",
			"module",
			"module/chunks/chunky",
			"crypto/hash",
			"crypto/random",
			"crypto/hash/ecc",
			"model/bootstrap",
			"model/bootstrap/info",
			"model",
		)
		cfg := loadPackagesConfig(configFile)
		matrices := buildTestMatrices(cfg, func(dir string) []*packages.Package {
			return allPackges
		})
		require.Len(t, matrices, 6)
		for _, matrix := range matrices {
			switch matrix.Name {
			case topLevelPkgName:
				require.Equal(t, defaultCIRunner, matrix.Runner)
				require.Equal(t, fmt.Sprintf("%s %s %s %s ", allPackges[0].PkgPath, allPackges[1].PkgPath, allPackges[2].PkgPath, allPackges[3].PkgPath), matrix.Packages)
			case subPkg1:
				require.Equal(t, subPkg1Runner, matrix.Runner)
				require.Equal(t, fmt.Sprintf("%s %s ", allPackges[4].PkgPath, allPackges[5].PkgPath), matrix.Packages)
			case subPkg2:
				require.Equal(t, defaultCIRunner, matrix.Runner)
				require.Equal(t, fmt.Sprintf("%s ", allPackges[7].PkgPath), matrix.Packages)
			case subPkg3:
				require.Equal(t, defaultCIRunner, matrix.Runner)
				require.Equal(t, fmt.Sprintf("%s %s ", allPackges[8].PkgPath, allPackges[10].PkgPath), matrix.Packages)
			case subPkg4:
				require.Equal(t, defaultCIRunner, matrix.Runner)
				require.Equal(t, fmt.Sprintf("%s %s ", allPackges[11].PkgPath, allPackges[12].PkgPath), matrix.Packages)
			case "others":
				require.Equal(t, defaultCIRunner, matrix.Runner)
				require.Equal(t, fmt.Sprintf("%s %s %s ", allPackges[6].PkgPath, allPackges[9].PkgPath, allPackges[13].PkgPath), matrix.Packages)
			default:
				require.Fail(t, fmt.Sprintf("unexpected matrix name: %s", matrix.Name))
			}
		}
	})
	t.Run("top level package with sub packages and exclude", func(t *testing.T) {
		topLevelPkgName := "network"
		subPkg1 := "network/p2p/node"
		subPkg1Runner := "buildjet-4vcpu-ubuntu-2204"
		configFile := fmt.Sprintf(`
			{"packagesPath": ".", "packages": [{"name": "%s", "exclude": ["network/alsp"], "subpackages": [{"name": "%s", "runner": "%s"}]}]}`,
			topLevelPkgName, subPkg1, subPkg1Runner)
		allPackges := goPackageFixture(
			"network",
			"network/alsp",
			"network/cache",
			"network/channels",
			"network/p2p/node",
			"network/p2p/node/internal",
			"module",
			"module/chunks/chunky",
			"crypto/hash",
			"crypto/random",
			"crypto/hash/ecc",
			"model/bootstrap",
			"model/bootstrap/info",
			"model",
		)
		cfg := loadPackagesConfig(configFile)
		matrices := buildTestMatrices(cfg, func(dir string) []*packages.Package {
			return allPackges
		})
		require.Len(t, matrices, 2)
		for _, matrix := range matrices {
			switch matrix.Name {
			case topLevelPkgName:
				require.Equal(t, defaultCIRunner, matrix.Runner)
				require.Equal(t, fmt.Sprintf("%s %s %s ", allPackges[0].PkgPath, allPackges[2].PkgPath, allPackges[3].PkgPath), matrix.Packages)
			case subPkg1:
				require.Equal(t, subPkg1Runner, matrix.Runner)
				require.Equal(t, fmt.Sprintf("%s %s ", allPackges[4].PkgPath, allPackges[5].PkgPath), matrix.Packages)
			default:
				require.Fail(t, fmt.Sprintf("unexpected matrix name: %s", matrix.Name))
			}
		}
	})
}

func goPackageFixture(pkgs ...string) []*packages.Package {
	goPkgs := make([]*packages.Package, len(pkgs))
	for i, pkg := range pkgs {
		goPkgs[i] = &packages.Package{PkgPath: fullGoPackagePath(pkg)}
	}
	return goPkgs
}
