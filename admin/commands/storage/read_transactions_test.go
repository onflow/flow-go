package storage

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/admin"
)

func TestReadTransactionsRangeTooWide(t *testing.T) {
	c := GetTransactionsCommand{}

	data := map[string]interface{}{
		"start-height": float64(1),
		"end-height":   float64(1001),
	}
	err := c.Validator(&admin.CommandRequest{
		Data: data,
	})
	require.Error(t, err)
	require.Contains(t, fmt.Sprintf("%v", err), "more than 1000 blocks")
}

func TestReadTransactionsRangeInvalid(t *testing.T) {
	c := GetTransactionsCommand{}

	data := map[string]interface{}{
		"start-height": float64(1001),
		"end-height":   float64(1000),
	}
	err := c.Validator(&admin.CommandRequest{
		Data: data,
	})
	require.Error(t, err)
	require.Contains(t, fmt.Sprintf("%v", err), "should not be smaller")
}

func TestReadTransactionsMissingStart(t *testing.T) {
	c := GetTransactionsCommand{}

	data := map[string]interface{}{
		"start-height": float64(1001),
	}
	err := c.Validator(&admin.CommandRequest{
		Data: data,
	})
	require.Error(t, err)
	require.Contains(t, fmt.Sprintf("%v", err), "end-height not set")
}

func TestReadTransactionsMissingEnd(t *testing.T) {
	c := GetTransactionsCommand{}

	data := map[string]interface{}{
		"end-height": float64(1001),
	}
	err := c.Validator(&admin.CommandRequest{
		Data: data,
	})
	require.Error(t, err)
	require.Contains(t, fmt.Sprintf("%v", err), "start-height not set")
}
