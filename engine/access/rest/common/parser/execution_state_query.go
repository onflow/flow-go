package parser

import (
	"fmt"
	"strconv"

	"github.com/onflow/flow-go/engine/access/rest/http/models"
	"github.com/onflow/flow-go/model/flow"
)

func NewExecutionStateQuery(
	agreeingExecutorsCount string,
	requiredExecutorIds []string,
	includeExecutorMetadata string,
) (*models.ExecutionStateQuery, error) {
	var executorCount uint64
	var err error
	if len(agreeingExecutorsCount) > 0 {
		executorCount, err = strconv.ParseUint(agreeingExecutorsCount, 10, 64)
		// executorCount can't be set to 0 explicitly
		if err != nil || executorCount == 0 {
			return nil, fmt.Errorf("invalid agreeingExecutorCount: %w", err)
		}
	}

	var ids IDs
	var executorIDs []flow.Identifier
	if len(requiredExecutorIds) > 0 {
		ids, err = NewIDs(requiredExecutorIds)
		if err != nil {
			return nil, fmt.Errorf("invalid requiredExecutorIds: %w", err)
		}
		executorIDs = ids.Flow()
	}

	var includeMetadata bool
	if len(includeExecutorMetadata) > 0 {
		includeMetadata, err = strconv.ParseBool(includeExecutorMetadata)
		if err != nil {
			return nil, fmt.Errorf("invalid includeExecutorMetadata: %w", err)
		}
	}

	return &models.ExecutionStateQuery{
		AgreeingExecutorsCount:  executorCount,
		RequiredExecutorIDs:     executorIDs,
		IncludeExecutorMetadata: includeMetadata,
	}, nil
}
