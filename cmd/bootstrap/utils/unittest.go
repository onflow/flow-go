package utils

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/utils/unittest"
)

func RunWithSporkBootstrapDir(t testing.TB, f func(bootDir, partnerDir, partnerWeights, internalPrivDir, configPath string)) {
	dir := unittest.TempDir(t)
	defer os.RemoveAll(dir)

	// make sure constraints are satisfied, 2/3's of con and col nodes are internal
	internalNodes := GenerateNodeInfos(3, 6, 2, 1, 1)
	partnerNodes := GenerateNodeInfos(1, 1, 1, 1, 1)

	partnerDir, partnerWeightsPath, err := WritePartnerFiles(partnerNodes, dir)
	require.NoError(t, err)

	internalPrivDir, configPath, err := WriteInternalFiles(internalNodes, dir)
	require.NoError(t, err)

	f(dir, partnerDir, partnerWeightsPath, internalPrivDir, configPath)
}
