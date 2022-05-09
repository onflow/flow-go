package debug_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/debug"
)

func TestDebugger_RunTransaction(t *testing.T) {

	grpcAddress := "35.206.124.52:9000"
	chain := flow.Emulator.Chain()
	debugger := debug.NewRemoteDebugger(grpcAddress, chain, zerolog.New(os.Stdout).With().Logger())

	const scriptTemplate = `
	import BloctoTokenStaking from 0x6e0797ac987005f5

	pub fun main(): UInt64 {
		return BloctoTokenStaking.getEpoch()
	}
	`

	script := []byte(fmt.Sprintf(scriptTemplate)

	// Run with blockID (use the file cache)
	blockId, err := flow.HexStringToIdentifier("cdffaba7feeecdf9d3686e54aea9582ed5ce9922cad16aae5722af3d8321e527")
	require.NoError(t, err)
	_, err, perr := debugger.RunScriptAtBlockID(script, [][]byte{}, blockId)
	require.NoError(t, perr)
	require.NoError(t, err)
}
