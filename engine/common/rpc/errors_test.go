package rpc

import (
	"fmt"
	"testing"

	"github.com/hashicorp/go-multierror"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gotest.tools/assert"
)

func TestConvertMultiError(t *testing.T) {
	defaultCode := codes.Internal
	t.Run("single error", func(t *testing.T) {
		var errors *multierror.Error
		errors = multierror.Append(errors, status.Error(codes.NotFound, "not found"))

		err := ConvertMultiError(errors, "", defaultCode)
		assert.Equal(t, codes.NotFound, status.Code(err))
	})

	t.Run("same code", func(t *testing.T) {
		var errors *multierror.Error
		errors = multierror.Append(errors, status.Error(codes.NotFound, "not found"))
		errors = multierror.Append(errors, status.Error(codes.NotFound, "not found"))
		errors = multierror.Append(errors, status.Error(codes.NotFound, "not found"))

		err := ConvertMultiError(errors, "", defaultCode)
		assert.Equal(t, codes.NotFound, status.Code(err))
	})

	t.Run("same codes - unavailable ignored", func(t *testing.T) {
		var errors *multierror.Error
		errors = multierror.Append(errors, status.Error(codes.NotFound, "not found"))
		errors = multierror.Append(errors, status.Error(codes.Unavailable, "unavailable"))
		errors = multierror.Append(errors, status.Error(codes.Unavailable, "unavailable"))

		err := ConvertMultiError(errors, "", defaultCode)
		assert.Equal(t, codes.NotFound, status.Code(err))
	})

	t.Run("same codes - deadline exceeded ignored", func(t *testing.T) {
		var errors *multierror.Error
		errors = multierror.Append(errors, status.Error(codes.DeadlineExceeded, "deadline exceeded"))
		errors = multierror.Append(errors, status.Error(codes.NotFound, "not found"))
		errors = multierror.Append(errors, status.Error(codes.NotFound, "not found"))

		err := ConvertMultiError(errors, "", defaultCode)
		assert.Equal(t, codes.NotFound, status.Code(err))
	})

	t.Run("all different codes", func(t *testing.T) {
		var errors *multierror.Error
		errors = multierror.Append(errors, status.Error(codes.NotFound, "not found"))
		errors = multierror.Append(errors, status.Error(codes.Internal, "internal"))
		errors = multierror.Append(errors, status.Error(codes.InvalidArgument, "invalid arg"))

		err := ConvertMultiError(errors, "", defaultCode)
		assert.Equal(t, defaultCode, status.Code(err))
	})

	t.Run("non-grpc errors", func(t *testing.T) {
		var errors *multierror.Error
		errors = multierror.Append(errors, status.Error(codes.NotFound, "not found"))
		errors = multierror.Append(errors, fmt.Errorf("not a grpc status code"))
		errors = multierror.Append(errors, status.Error(codes.NotFound, "not found"))

		err := ConvertMultiError(errors, "some prefix", defaultCode)
		assert.Equal(t, defaultCode, status.Code(err))
		assert.ErrorContains(t, err, "some prefix: ")
	})
}
