package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Can't have a const []string so resorting to using a test helper function.
func getAllFlowPackages() []string {
	return []string{
		flowPackagePrefix + "abc",
		flowPackagePrefix + "abc/123",
		flowPackagePrefix + "abc/def",
		flowPackagePrefix + "abc/def/ghi",
		flowPackagePrefix + "def",
		flowPackagePrefix + "def/abc",
		flowPackagePrefix + "ghi",
		flowPackagePrefix + "jkl",
		flowPackagePrefix + "mno/abc",
		flowPackagePrefix + "pqr",
		flowPackagePrefix + "stu",
		flowPackagePrefix + "vwx",
		flowPackagePrefix + "vwx/ghi",
		flowPackagePrefix + "yz",
	}
}

// TestListTargetPackages_DefaultRunners tests that the target packages are included in the target packages and seen packages.
// All packages use default CI runners.
func TestListTargetPackages_DefaultRunners(t *testing.T) {
	targetPackages, seenPackages := listTargetPackages([]string{"abc", "ghi"}, getAllFlowPackages())
	require.Equal(t, 2, len(targetPackages.packages))

	// check all TARGET packages
	// there should be 4 packages that start with "abc"
	require.Equal(t, 4, len(targetPackages.packages["abc"]))
	require.Contains(t, targetPackages.packages["abc"], flowPackagePrefix+"abc")
	require.Contains(t, targetPackages.packages["abc"], flowPackagePrefix+"abc/123")
	require.Contains(t, targetPackages.packages["abc"], flowPackagePrefix+"abc/def")
	require.Contains(t, targetPackages.packages["abc"], flowPackagePrefix+"abc/def/ghi")

	// there should be 1 package that starts with "ghi"
	require.Equal(t, 1, len(targetPackages.packages["ghi"]))
	require.Contains(t, targetPackages.packages["ghi"], flowPackagePrefix+"ghi")

	// check all CI RUNNERS for each target package
	require.Equal(t, 2, len(targetPackages.runners))
	require.Equal(t, targetPackages.runners["abc"], ciDefaultRunner)
	require.Equal(t, targetPackages.runners["ghi"], ciDefaultRunner)

	// check all SEEN packages
	// there should be 5 packages that start with "abc" or "ghi"
	require.Equal(t, 5, len(seenPackages))
	require.Contains(t, seenPackages, flowPackagePrefix+"abc")
	require.Contains(t, seenPackages, flowPackagePrefix+"abc/123")
	require.Contains(t, seenPackages, flowPackagePrefix+"abc/def")
	require.Contains(t, seenPackages, flowPackagePrefix+"abc/def/ghi")
	require.Contains(t, seenPackages, flowPackagePrefix+"ghi")
}

// TestListTargetSubPackages_CustomRunners tests that if a subpackage is specified as a target package, then the sub package and
// all children of the sub package are also included in the target packages.
func TestListTargetSubPackages_CustomRunners(t *testing.T) {
	targetPackages, seenPackages := listTargetPackages([]string{"abc/def:foo_runner"}, getAllFlowPackages())
	require.Equal(t, 1, len(targetPackages.packages))

	// check all TARGET packages
	// there should be 2 target subpackages that starts with "abc/def"
	require.Equal(t, 2, len(targetPackages.packages["abc/def"]))
	require.Contains(t, targetPackages.packages["abc/def"], flowPackagePrefix+"abc/def")
	require.Contains(t, targetPackages.packages["abc/def"], flowPackagePrefix+"abc/def/ghi")

	// check all CI RUNNERS for each target package
	require.Equal(t, 1, len(targetPackages.runners))
	require.Equal(t, targetPackages.runners["abc/def"], "foo_runner")

	// check all SEEN packages
	// there should be 2 seen subpackages that start with "abc/def"
	require.Equal(t, 2, len(seenPackages))
	require.Contains(t, seenPackages, flowPackagePrefix+"abc/def")
	require.Contains(t, seenPackages, flowPackagePrefix+"abc/def/ghi")
}

// TestListOtherPackages tests that the remaining packages that don't match any of the target packages are included
func TestListOtherPackages(t *testing.T) {
	var seenPackages = make(map[string]string)
	seenPackages[flowPackagePrefix+"abc/def"] = flowPackagePrefix + "abc/def"
	seenPackages[flowPackagePrefix+"abc/def/ghi"] = flowPackagePrefix + "abc/def/ghi"
	seenPackages[flowPackagePrefix+"ghi"] = flowPackagePrefix + "ghi"
	seenPackages[flowPackagePrefix+"mno/abc"] = flowPackagePrefix + "mno/abc"
	seenPackages[flowPackagePrefix+"stu"] = flowPackagePrefix + "stu"

	otherPackages := listOtherPackages(getAllFlowPackages(), seenPackages)

	require.Equal(t, 9, len(otherPackages))

	require.Contains(t, otherPackages, flowPackagePrefix+"abc")
	require.Contains(t, otherPackages, flowPackagePrefix+"abc/123")
	require.Contains(t, otherPackages, flowPackagePrefix+"def")
	require.Contains(t, otherPackages, flowPackagePrefix+"def/abc")
	require.Contains(t, otherPackages, flowPackagePrefix+"jkl")
	require.Contains(t, otherPackages, flowPackagePrefix+"pqr")
	require.Contains(t, otherPackages, flowPackagePrefix+"vwx")
	require.Contains(t, otherPackages, flowPackagePrefix+"vwx/ghi")
	require.Contains(t, otherPackages, flowPackagePrefix+"yz")
}

// TestGenerateTestMatrix tests that the test matrix is generated correctly where the target packages include top level
// packages as well as sub packages.
func TestGenerateTestMatrix(t *testing.T) {
	targetPackages, seenPackages := listTargetPackages([]string{"abc/def", "def:foo-runner", "ghi"}, getAllFlowPackages())
	require.Equal(t, 3, len(targetPackages.packages))
	require.Equal(t, 5, len(seenPackages))

	// check that the target packages have correct CI runners
	require.Equal(t, 3, len(targetPackages.runners))
	require.Equal(t, targetPackages.runners["abc/def"], "ubuntu-latest")
	require.Equal(t, targetPackages.runners["def"], "foo-runner")
	require.Equal(t, targetPackages.runners["ghi"], "ubuntu-latest")

	otherPackages := listOtherPackages(getAllFlowPackages(), seenPackages)

	matrix := generateTestMatrix(targetPackages, otherPackages)

	// should be 3 groups in test matrix: abc/def, def, ghi, others
	require.Equal(t, 4, len(matrix))

	require.Contains(t, matrix, testMatrix{
		Name:     "abc/def",
		Packages: "github.com/onflow/flow-go/abc/def github.com/onflow/flow-go/abc/def/ghi",
		Runner:   "ubuntu-latest"},
	)
	require.Contains(t, matrix, testMatrix{
		Name:     "def",
		Packages: "github.com/onflow/flow-go/def github.com/onflow/flow-go/def/abc",
		Runner:   "foo-runner"},
	)
	require.Contains(t, matrix, testMatrix{
		Name:     "ghi",
		Packages: "github.com/onflow/flow-go/ghi",
		Runner:   "ubuntu-latest"},
	)
	require.Contains(t, matrix, testMatrix{
		Name:     "others",
		Packages: "github.com/onflow/flow-go/abc github.com/onflow/flow-go/abc/123 github.com/onflow/flow-go/jkl github.com/onflow/flow-go/mno/abc github.com/onflow/flow-go/pqr github.com/onflow/flow-go/stu github.com/onflow/flow-go/vwx github.com/onflow/flow-go/vwx/ghi github.com/onflow/flow-go/yz",
		Runner:   "ubuntu-latest"},
	)
}
