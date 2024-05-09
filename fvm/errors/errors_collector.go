package errors

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
)

// Utility object for collecting Processor errors.
type ErrorsCollector struct {
	errors  error
	failure error
}

func NewErrorsCollector() *ErrorsCollector {
	return &ErrorsCollector{}
}

func (collector *ErrorsCollector) CollectedFailure() bool {
	return collector.failure != nil
}

func (collector *ErrorsCollector) CollectedError() bool {
	return collector.errors != nil || collector.failure != nil
}

func (collector *ErrorsCollector) ErrorOrNil() error {
	if collector.failure == nil {
		return collector.errors
	}

	if collector.errors == nil {
		return collector.failure
	}

	return fmt.Errorf(
		"%w (additional errors: %v)",
		collector.failure,
		collector.errors)
}

// This returns collector for chaining purposes.
func (collector *ErrorsCollector) Collect(err error) *ErrorsCollector {
	if err == nil {
		return collector
	}

	if collector.failure != nil {
		collector.errors = multierror.Append(collector.errors, err)
		return collector
	}

	if IsFailure(err) {
		collector.failure = err
		return collector
	}

	collector.errors = multierror.Append(collector.errors, err)
	return collector
}
