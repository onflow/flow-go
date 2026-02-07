package filter

import (
	"fmt"

	"github.com/onflow/cadence"
	"github.com/onflow/flow-go/model/flow"
)

type MatchOperation string

const (
	MatchOperationEqual              MatchOperation = "EQUAL"                 // any type
	MatchOperationNotEqual           MatchOperation = "NOT_EQUAL"             // any type
	MatchOperationGreaterThan        MatchOperation = "GREATER_THAN"          // any numeric type
	MatchOperationLessThan           MatchOperation = "LESS_THAN"             // any numeric type
	MatchOperationGreaterThanOrEqual MatchOperation = "GREATER_THAN_OR_EQUAL" // any numeric type
	MatchOperationLessThanOrEqual    MatchOperation = "LESS_THAN_OR_EQUAL"    // any numeric type

	// TODO: complete implementation and testing
	MatchOperationContains       MatchOperation = "CONTAINS"         // string, array, dictionary. allow string/value/array of values
	MatchOperationDoesNotContain MatchOperation = "DOES_NOT_CONTAIN" // string, array, dictionary. allow string/value/array of values
	MatchOperationIn             MatchOperation = "IN"               // any type. allow array of values
	MatchOperationNotIn          MatchOperation = "NOT_IN"           // any type. allow array of values
)

var (
	allowedMatchOperators = map[MatchOperation]struct{}{
		MatchOperationEqual:              {},
		MatchOperationNotEqual:           {},
		MatchOperationGreaterThan:        {},
		MatchOperationLessThan:           {},
		MatchOperationGreaterThanOrEqual: {},
		MatchOperationLessThanOrEqual:    {},
	}
)

// Only allow filtering fields numeric, string, array, optional

type TypedFieldFilter struct {
	FieldName string
	Operation MatchOperation
	Value     cadence.Value

	// TODO: is this the best way to handle this?
	AllowedValues []cadence.Value
}

var _ Matcher = (*TypedFieldFilter)(nil)

func NewTypedFieldFilter(fieldName string, operation MatchOperation, value cadence.Value, allowedValues []cadence.Value) (*TypedFieldFilter, error) {
	if fieldName == "" {
		return nil, fmt.Errorf("field name must not be empty")
	}
	if _, ok := allowedMatchOperators[operation]; !ok {
		return nil, fmt.Errorf("field name must not be empty")
	}
	if !isAllowedFilterType(value) {
		return nil, fmt.Errorf("unsupported filter by type %s", value.Type().ID())
	}

	// make sure all values have the same type
	var valueType cadence.Type
	for _, allowedValue := range allowedValues {
		if !isAllowedFilterType(allowedValue) {
			return nil, fmt.Errorf("unsupported filter by type %s", allowedValue.Type().ID())
		}

		if valueType == nil { // TODO: not sure this will work since valueType is an interface
			valueType = allowedValue.Type()
		}
		ok, err := typesEqual(allowedValue, valueType)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("type of values in allowedValues must match filter type %s", value.Type().ID())
		}
	}

	return &TypedFieldFilter{
		FieldName:     fieldName,
		Operation:     operation,
		Value:         value,
		AllowedValues: allowedValues,
	}, nil
}

func (f *TypedFieldFilter) Match(event *flow.Event) (bool, error) {
	fields, err := getEventFields(event)
	if err != nil {
		return false, fmt.Errorf("error getting event fields: %w", err)
	}

	fieldValue, ok := fields[f.FieldName]
	if !ok {
		return false, nil
	}

	ok, err = typesEqual(fieldValue, f.Value.Type())
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}

	switch f.Operation {
	case MatchOperationEqual:
		return equal(fieldValue, f.Value)

	case MatchOperationNotEqual:
		equal, err := equal(fieldValue, f.Value)
		if err != nil {
			return false, err
		}
		return !equal, nil

	case MatchOperationGreaterThan:
		cmp, err := cmp(fieldValue, f.Value)
		if err != nil {
			return false, err
		}
		return cmp > 0, nil

	case MatchOperationLessThan:
		cmp, err := cmp(fieldValue, f.Value)
		if err != nil {
			return false, err
		}
		return cmp < 0, nil

	case MatchOperationGreaterThanOrEqual:
		cmp, err := cmp(fieldValue, f.Value)
		if err != nil {
			return false, err
		}
		return cmp >= 0, nil

	case MatchOperationLessThanOrEqual:
		cmp, err := cmp(fieldValue, f.Value)
		if err != nil {
			return false, err
		}
		return cmp <= 0, nil

	case MatchOperationContains:
		return contains(fieldValue, f.Value)

	case MatchOperationDoesNotContain:
		contains, err := contains(fieldValue, f.Value)
		if err != nil {
			return false, err
		}
		return !contains, nil

	case MatchOperationIn:
		if len(f.AllowedValues) == 0 {
			return false, nil
		}

		return in(fieldValue, f.AllowedValues)

	case MatchOperationNotIn:
		in, err := in(fieldValue, f.AllowedValues)
		if err != nil {
			return false, err
		}
		return !in, nil

	default:
		return false, fmt.Errorf("unsupported filter operation: %s", f.Operation)
	}
}
