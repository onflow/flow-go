package query_mode

import (
	"errors"
)

type IndexQueryMode int

const (
	// IndexQueryModeLocalOnly executes scripts and gets accounts using only local storage
	IndexQueryModeLocalOnly IndexQueryMode = iota + 1

	// IndexQueryModeExecutionNodesOnly executes scripts and gets accounts using only
	// execution nodes
	IndexQueryModeExecutionNodesOnly

	// IndexQueryModeFailover executes scripts and gets accounts using local storage first,
	// then falls back to execution nodes if data is not available for the height or if request
	// failed due to a non-user error.
	IndexQueryModeFailover
)

func ParseIndexQueryMode(s string) (IndexQueryMode, error) {
	switch s {
	case IndexQueryModeLocalOnly.String():
		return IndexQueryModeLocalOnly, nil
	case IndexQueryModeExecutionNodesOnly.String():
		return IndexQueryModeExecutionNodesOnly, nil
	case IndexQueryModeFailover.String():
		return IndexQueryModeFailover, nil
	default:
		return 0, errors.New("invalid script execution mode")
	}
}

func (m IndexQueryMode) String() string {
	switch m {
	case IndexQueryModeLocalOnly:
		return "local-only"
	case IndexQueryModeExecutionNodesOnly:
		return "execution-nodes-only"
	case IndexQueryModeFailover:
		return "failover"
	default:
		return ""
	}
}
