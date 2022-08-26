package main

import "testing"
import "github.com/stretchr/testify/require"

func TestListRestPackages(t *testing.T) {
	allFlowPackages := []string{
		"abc",
		"def",
		"ghi",
		"jkl",
		"mno",
		"pqr",
		"stu",
		"vwxyx",
	}

	var seenPackages = make(map[string]string)
	seenPackages["abc"] = "abc"
	seenPackages["ghi"] = "ghi"
	seenPackages["mno"] = "mno"
	seenPackages["stu"] = "stu"

	restPackages := listRestPackages(allFlowPackages, seenPackages)

	require.Equal(t, 4, len(restPackages))
	require.Contains(t, restPackages, "def")
	require.Contains(t, restPackages, "jkl")
	require.Contains(t, restPackages, "pqr")
	require.Contains(t, restPackages, "vwxyx")
}
