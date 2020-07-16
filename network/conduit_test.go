package network

import (
	"fmt"
	"testing"

	"github.com/hashicorp/go-multierror"
	"github.com/stretchr/testify/require"
)

func TestWrappedByMultiError(t *testing.T) {
	var errs *multierror.Error

	err := NewPeerUnreachableError(fmt.Errorf("unreachable"))
	errs = multierror.Append(errs, fmt.Errorf("could not send req: %w", err))

	require.True(t, IsPeerUnreachableError(err))                     // Pass
	require.True(t, IsPeerUnreachableError(errs.WrappedErrors()[0])) // Pass
}

func TestNestedWrappedMultiError(t *testing.T) {
	var innerError *multierror.Error
	innerError = multierror.Append(innerError, fmt.Errorf("inner error"))
	innerError = multierror.Append(innerError, fmt.Errorf("inner error"))
	innerError = multierror.Append(innerError, fmt.Errorf("inner error"))

	err := NewPeerUnreachableError(fmt.Errorf("inner: %w", innerError))
	require.True(t, IsPeerUnreachableError(err))
	var outerError *multierror.Error
	outerError = multierror.Append(outerError, fmt.Errorf("inner: %w", err))
	require.True(t, AllPeerUnreachableError(outerError.WrappedErrors()...))
}
