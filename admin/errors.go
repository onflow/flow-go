package admin

import (
	"errors"
	"fmt"
)

var ErrValidatorReqDataFormat error = errors.New("wrong input format: expected JSON")

type InvalidAdminParameterError struct {
	field     string
	msg       string
	actualVal interface{}
}

func NewInvalidAdminParameterError(field string, msg string, actualVal any) *InvalidAdminParameterError {
	return &InvalidAdminParameterError{
		field:     field,
		msg:       fmt.Sprintf("invalid value for '%s': %s. Got: %v", field, msg, actualVal),
		actualVal: actualVal,
	}
}

func IsInvalidAdminParameterError(err error) bool {
	var target *InvalidAdminParameterError
	return errors.As(err, &target)
}

func (i *InvalidAdminParameterError) Error() string {
	return i.msg
}

func (i *InvalidAdminParameterError) Field() string {
	return i.field
}

func (i *InvalidAdminParameterError) ActualVal() any {
	return i.actualVal
}
