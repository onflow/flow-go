package main

import "testing"
import "github.com/stretchr/testify/require"

// Can't have a const []string so resorting to using a test helper function.
func getAllFlowPackages() []string {
	return []string{
		flowPackagePrefix + "abc",
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

func TestListTargetPackages(t *testing.T) {
	targetPackages, seenPackages := listTargetPackages([]string{"abc", "ghi"}, getAllFlowPackages())
	require.Equal(t, 2, len(targetPackages))
	require.Equal(t, 4, len(seenPackages))

	// there should be 3 packages that start with "abc"
	require.Equal(t, 3, len(targetPackages["abc"]))
	require.Contains(t, targetPackages["abc"], flowPackagePrefix+"abc")
	require.Contains(t, targetPackages["abc"], flowPackagePrefix+"abc/def")
	require.Contains(t, targetPackages["abc"], flowPackagePrefix+"abc/def/ghi")

	// there should be 1 package that starts with "ghi"
	require.Equal(t, 1, len(targetPackages["ghi"]))
	require.Contains(t, targetPackages["ghi"], flowPackagePrefix+"ghi")

	require.Contains(t, seenPackages, flowPackagePrefix+"abc")
	require.Contains(t, seenPackages, flowPackagePrefix+"abc/def")
	require.Contains(t, seenPackages, flowPackagePrefix+"abc/def/ghi")
	require.Contains(t, seenPackages, flowPackagePrefix+"ghi")
}

func TestListRestPackages(t *testing.T) {
	var seenPackages = make(map[string]string)
	seenPackages[flowPackagePrefix+"abc"] = flowPackagePrefix + "abc"
	seenPackages[flowPackagePrefix+"ghi"] = flowPackagePrefix + "ghi"
	seenPackages[flowPackagePrefix+"mno/abc"] = flowPackagePrefix + "mno/abc"
	seenPackages[flowPackagePrefix+"stu"] = flowPackagePrefix + "stu"

	restPackages := listRestPackages(getAllFlowPackages(), seenPackages)

	require.Equal(t, 9, len(restPackages))

	require.Contains(t, restPackages, flowPackagePrefix+"abc/def")
	require.Contains(t, restPackages, flowPackagePrefix+"abc/def/ghi")
	require.Contains(t, restPackages, flowPackagePrefix+"def")
	require.Contains(t, restPackages, flowPackagePrefix+"def/abc")
	require.Contains(t, restPackages, flowPackagePrefix+"jkl")
	require.Contains(t, restPackages, flowPackagePrefix+"pqr")
	require.Contains(t, restPackages, flowPackagePrefix+"vwx")
	require.Contains(t, restPackages, flowPackagePrefix+"vwx/ghi")
	require.Contains(t, restPackages, flowPackagePrefix+"yz")
}

func TestGenerateTestMatrix(t *testing.T) {
	targetPackages, seenPackages := listTargetPackages([]string{"abc", "ghi"}, getAllFlowPackages())
	require.Equal(t, 2, len(targetPackages))
	require.Equal(t, 4, len(seenPackages))

	restPackages := listRestPackages(getAllFlowPackages(), seenPackages)

	testMatrix := generateTestMatrix(targetPackages, restPackages)

	// should be 3 groups in test matrix: abc, ghi, rest
	require.Equal(t, 3, len(testMatrix))

	require.Equal(t, "abc", testMatrix[0].Name)
	require.Equal(t, "github.com/onflow/flow-go/abc github.com/onflow/flow-go/abc/def github.com/onflow/flow-go/abc/def/ghi",
		testMatrix[0].Packages)

	require.Equal(t, "ghi", testMatrix[1].Name)
	require.Equal(t, "github.com/onflow/flow-go/ghi", testMatrix[1].Packages)

	require.Equal(t, "rest", testMatrix[2].Name)
	require.Equal(t, "github.com/onflow/flow-go/def github.com/onflow/flow-go/def/abc github.com/onflow/flow-go/jkl github.com/onflow/flow-go/mno/abc github.com/onflow/flow-go/pqr github.com/onflow/flow-go/stu github.com/onflow/flow-go/vwx github.com/onflow/flow-go/vwx/ghi github.com/onflow/flow-go/yz",
		testMatrix[2].Packages)
}
