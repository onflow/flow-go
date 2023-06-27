package rpc

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/go-multierror"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/storage"
)

func TestConvertError(t *testing.T) {
	defaultCode := codes.Internal
	t.Run("no error", func(t *testing.T) {
		err := ConvertError(nil, "", defaultCode)
		assert.NoError(t, err)
	})

	t.Run("preset status code", func(t *testing.T) {
		err := ConvertError(status.Error(codes.Unavailable, "Unavailable"), "", defaultCode)
		assert.Equal(t, codes.Unavailable, status.Code(err))

		err = ConvertError(status.Error(codes.OutOfRange, "OutOfRange"), "", defaultCode)
		assert.Equal(t, codes.OutOfRange, status.Code(err))

		err = ConvertError(status.Error(codes.Internal, "Internal"), "", defaultCode)
		assert.Equal(t, codes.Internal, status.Code(err))
	})

	t.Run("multierror", func(t *testing.T) {
		var errors *multierror.Error
		errors = multierror.Append(errors, status.Error(codes.NotFound, "not found"))
		errors = multierror.Append(errors, fmt.Errorf("not a grpc status code"))
		errors = multierror.Append(errors, status.Error(codes.NotFound, "not found"))

		err := ConvertError(errors, "some prefix", defaultCode)
		assert.Equal(t, defaultCode, status.Code(err))
		assert.ErrorContains(t, err, "some prefix: ")
	})

	t.Run("derived code", func(t *testing.T) {
		err := ConvertError(context.Canceled, "", defaultCode)
		assert.Equal(t, codes.Canceled, status.Code(err))

		err = ConvertError(context.DeadlineExceeded, "some prefix", defaultCode)
		assert.Equal(t, codes.DeadlineExceeded, status.Code(err))
		assert.ErrorContains(t, err, "some prefix: ")
	})

	t.Run("unhandled code", func(t *testing.T) {
		err := ConvertError(storage.ErrNotFound, "", defaultCode)
		assert.Equal(t, codes.Internal, status.Code(err))

		err = ConvertError(status.Error(codes.Unknown, "Unknown"), "", defaultCode)
		assert.Equal(t, codes.Internal, status.Code(err))

		err = ConvertError(status.Error(codes.Internal, "Internal"), "", defaultCode)
		assert.Equal(t, codes.Internal, status.Code(err))

		err = ConvertError(fmt.Errorf("unhandled error"), "", defaultCode)
		assert.Equal(t, codes.Internal, status.Code(err))

		err = ConvertError(fmt.Errorf("unhandled error"), "some prefix", defaultCode)
		assert.Equal(t, defaultCode, status.Code(err))
		assert.ErrorContains(t, err, "some prefix: ")
	})
}

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
