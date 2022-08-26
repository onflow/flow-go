package main

import "testing"
import "github.com/stretchr/testify/require"

// Can't have a const []string so resorting to using a test helper function.
func getAllFlowPackages() []string {
	return []string{
		"abc",
		"def",
		"ghi",
		"jkl",
		"mno",
		"pqr",
		"stu",
		"vwx",
		"yz",
	}
}

func TestListTargetPackages(t *testing.T) {

}

func TestListRestPackages(t *testing.T) {
	var seenPackages = make(map[string]string)
	seenPackages["abc"] = "abc"
	seenPackages["ghi"] = "ghi"
	seenPackages["mno"] = "mno"
	seenPackages["stu"] = "stu"

	restPackages := listRestPackages(getAllFlowPackages(), seenPackages)

	require.Equal(t, 5, len(restPackages))
	require.Contains(t, restPackages, "def")
	require.Contains(t, restPackages, "jkl")
	require.Contains(t, restPackages, "pqr")
	require.Contains(t, restPackages, "vwx")
	require.Contains(t, restPackages, "yz")
}
