package filter

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

type GroupingOperation string

const (
	GroupingOperationAnd GroupingOperation = "AND"
	GroupingOperationOr  GroupingOperation = "OR"
	GroupingOperationNot GroupingOperation = "NOT"
)

type FieldFilterSet struct {
	GroupingOperation GroupingOperation
	Filters           []Matcher
}

var _ Matcher = (*FieldFilterSet)(nil)

func NewFieldFilterSet(groupingOperation GroupingOperation, filters []Matcher) (*FieldFilterSet, error) {
	if len(filters) == 0 {
		return nil, fmt.Errorf("filter set cannot be empty")
	}

	return &FieldFilterSet{
		GroupingOperation: groupingOperation,
		Filters:           filters,
	}, nil
}

func (f *FieldFilterSet) Match(event *flow.Event) (bool, error) {
	matchCount := 0
	for i, filter := range f.Filters {
		matched, err := filter.Match(event)
		if err != nil {
			return false, fmt.Errorf("error matching filter %d: %w", i, err)
		}

		switch f.GroupingOperation {
		case GroupingOperationOr:
			if matched {
				return true, nil
			}
		case GroupingOperationAnd:
			if !matched {
				return false, nil
			}
		case GroupingOperationNot:
			if matched {
				return false, nil
			}
		}
	}

	switch f.GroupingOperation {
	case GroupingOperationAnd:
		return matchCount == len(f.Filters), nil
	case GroupingOperationOr:
		return matchCount > 0, nil
	case GroupingOperationNot:
		return matchCount == 0, nil
	}
	return false, nil
}
