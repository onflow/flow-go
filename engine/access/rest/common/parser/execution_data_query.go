package parser

import (
	"fmt"
	"strconv"
)

func NewExecutionDataQuery(
	agreeingExecutorCount string,
	requiredExecutorIds []string,
	includeExecutorMetadata string,
) (executorCount uint64, executorIDs [][]byte, includeMetadata bool, err error) {
	// 1. Parse agreeingExecutorCount -> uint64
	if len(agreeingExecutorCount) > 0 {
		executorCount, err = strconv.ParseUint(agreeingExecutorCount, 10, 64)
		// executorCount can't be set to 0 explicitly
		if err != nil || executorCount == 0 {
			return 0, nil, false, fmt.Errorf("invalid agreeingExecutorCount: %w", err)
		}
	}

	// 2. Parse requiredExecutorIds -> []flow.Identifier
	var ids IDs
	if len(requiredExecutorIds) > 0 {
		ids, err = NewIDs(requiredExecutorIds)
		if err != nil {
			return 0, nil, false, fmt.Errorf("invalid requiredExecutorIds: %w", err)
		}
	}

	// cast every flow.Identifier to []byte
	executorIDs = make([][]byte, len(ids.Flow()))
	for i, id := range ids.Flow().Strings() {
		executorIDs[i] = []byte(id)
	}

	// 3. Parse includeExecutorMetadata -> bool
	if len(includeExecutorMetadata) > 0 {
		includeMetadata, err = strconv.ParseBool(includeExecutorMetadata)
		if err != nil {
			return 0, nil, false, fmt.Errorf("invalid includeExecutorMetadata: %w", err)
		}
	}

	return executorCount, executorIDs, includeMetadata, nil
}
