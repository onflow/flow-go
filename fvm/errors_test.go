package fvm_test

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Error_Handling(t *testing.T) {

	fvm.HandleError()
	require.NoError(t, err)
	require.Equal(t, header0, *heightFrom)

}
