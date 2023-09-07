package storage

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/admin"
)

func TestReadTransactionsRangeTooWide(t *testing.T) {
	c := GetTransactionsCommand{}

	data := map[string]interface{}{
		"start-height": float64(1),
		"end-height":   float64(10002),
	}

	req := &admin.CommandRequest{
		Data: data,
	}
	err := c.Validator(req)
	require.NoError(t, err)

	_, err = c.Handler(context.Background(), req)
	require.Error(t, err)
	require.Contains(t, fmt.Sprintf("%v", err), "more than 10001 blocks")
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
	require.True(t, admin.IsInvalidAdminParameterError(err))
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
	require.True(t, admin.IsInvalidAdminParameterError(err))
}
