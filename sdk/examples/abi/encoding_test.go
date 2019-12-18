package abi_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/language/runtime/cmd/abi"
	"github.com/dapperlabs/flow-go/protobuf/services/observation"
	"github.com/dapperlabs/flow-go/sdk/abi/generation/code"
	"github.com/dapperlabs/flow-go/sdk/abi/types"
	"github.com/dapperlabs/flow-go/sdk/emulator"
	"github.com/dapperlabs/flow-go/sdk/emulator/server"
	"github.com/dapperlabs/flow-go/sdk/examples/abi/generated"
)

func TestDecodingUsingAbi(t *testing.T) {
	// Generate JSON ABI
	const cadenceFilename = "music.cdc"
	cadenceFile, err := ioutil.ReadFile(cadenceFilename)
	require.NoError(t, err)

	abiJson := abi.GetABIJSONFromCadenceCode(string(cadenceFile), true, cadenceFilename)

	abiFilename := "generated/music.cdc.abi.json"
	goFilename := "generated/music.gen.go"
	abiFile, err := os.Create(abiFilename)
	if err != nil {
		require.NoError(t, err)
	}

	_, err = abiFile.Write(abiJson)
	require.NoError(t, err)

	// Generate Go code from
	// TODO This is some sort of chicken and egg problem, when we need generated Go code to compile
	// tests. Ideally, we will have SDK tools able to generate the code in separate build step
	// but for now, we just keep all calls internal.
	// It still should help us find problem in this flow

	allTypes, err := abi.GetTypesFromABIJSONBytes(abiJson)

	if err != nil {
		require.NoError(t, err)
	}

	goFile, err := os.Create(goFilename)
	if err != nil {
		require.NoError(t, err)
	}

	compositeTypes := map[string]*types.Composite{}
	for name, typ := range allTypes {
		switch composite := typ.(type) {
		case *types.Resource:
			compositeTypes[name] = &composite.Composite
		case *types.Struct:
			compositeTypes[name] = &composite.Composite
		default:
			_, err := fmt.Fprintf(os.Stderr, "Definition %s of type %T is not supported, skipping\n", name, typ)
			if err != nil {
				require.NoError(t, err)
			}
		}

	}

	err = code.GenerateGo("generated", compositeTypes, goFile)
	if err != nil {
		require.NoError(t, err)
	}

	emulator, err := emulator.NewEmulatedBlockchain()
	require.NoError(t, err)

	backend := server.NewBackend(emulator, logrus.New())

	ctx := context.Background()

	response, err := backend.ExecuteScript(ctx, &observation.ExecuteScriptRequest{
		Script: cadenceFile,
	})

	require.NoError(t, err)

	albums, err := generated.DecodeAlbumViewVariableSizedArray(response.Value)

	require.NoError(t, err)

	// Those values come from hardcoded function in music.cdc
	assert.Len(t, albums, 3)
	assert.NotEmpty(t, albums[0].Artist())
	assert.NotNil(t, albums[1].Artist().Members())
	assert.Equal(t, "Ralf Hütter", (*albums[1].Artist().Members())[0])

	assert.Nil(t, albums[0].Rating())
	assert.Nil(t, albums[2].Artist().Members())
}
