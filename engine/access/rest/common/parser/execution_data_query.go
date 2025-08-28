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
	//TODO: all of the passed args can be empty. Do we handle such a case?

	// 1. Parse agreeingExecutorCount -> uint64
	count, err := strconv.ParseUint(agreeingExecutorCount, 10, 64)
	if err != nil || count == 0 {
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

	// cast every flow.Identifier to []byte
	byteIDs := make([][]byte, len(ids.Flow()))
	for i, id := range ids.Flow() {
		byteIDs[i] = []byte(id.String())
	}

	return count, byteIDs, include, nil
}
