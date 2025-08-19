package parser

import (
	"fmt"
	"strconv"
)

func NewExecutionDataQuery(
	agreeingExecutorCount string,
	requiredExecutorIds []string,
	includeExecutorMetadata string,
) (uint64, [][]byte, bool, error) {
	// 1. Parse agreeingExecutorCount -> uint64
	count, err := strconv.ParseUint(agreeingExecutorCount, 10, 64)
	if err != nil {
		return 0, nil, false, fmt.Errorf("invalid agreeingExecutorCount: %w", err)
	}

	// 2. Parse requiredExecutorIds -> []flow.Identifier
	ids, err := NewIDs(requiredExecutorIds)
	if err != nil {
		return 0, nil, false, fmt.Errorf("invalid requiredExecutorIds: %w", err)
	}

	// 3. Parse includeExecutorMetadata -> bool
	include, err := strconv.ParseBool(includeExecutorMetadata)
	if err != nil {
		return 0, nil, false, fmt.Errorf("invalid includeExecutorMetadata: %w", err)
	}

	return count, ids.Bytes(), include, nil
}
