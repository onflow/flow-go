package fork

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

func IncludingBlock(lowestBlockId flow.Identifier) Terminal {
	return terminalBlock{lowestBlockId, true}
}

func ExcludingBlock(lowestBlockId flow.Identifier) Terminal {
	return terminalBlock{lowestBlockId, false}
}

type terminalBlock struct {
	terminalBlockID flow.Identifier
	inclusive       bool
}

func (t terminalBlock) Translate2Height(headers storage.Headers) (uint64, sanityCheckLowestVisitedBlock, error) {
	terminalHeader, err := headers.ByBlockID(t.terminalBlockID)
	if err != nil {
		return 0, noopSanityChecker, fmt.Errorf("failed to retrieve header of terminal block %x: %w", t.terminalBlockID, err)
	}

	if t.inclusive {
		sanityChecker := func(header *flow.Header) error {
			if header.ID() != t.terminalBlockID {
				return fmt.Errorf("last visited block has ID %x but expecting %x", header.ID(), t.terminalBlockID)
			}
			return nil
		}
		return terminalHeader.Height, sanityChecker, nil
	} else {
		sanityChecker := func(header *flow.Header) error {
			if header.ParentID != t.terminalBlockID {
				return fmt.Errorf("parent of last visited block has ID %x but expecting %x", header.ParentID, t.terminalBlockID)
			}
			return nil
		}
		return terminalHeader.Height + 1, sanityChecker, nil
	}
}

func IncludingHeight(lowestHeight uint64) Terminal {
	return terminalHeight{lowestHeight}
}

func ExcludingHeight(lowestHeight uint64) Terminal {
	return terminalHeight{lowestHeight + 1}
}

type terminalHeight struct {
	lowestHeightToVisit uint64
}

func (t terminalHeight) Translate2Height(storage.Headers) (uint64, sanityCheckLowestVisitedBlock, error) {
	return t.lowestHeightToVisit, noopSanityChecker, nil
}

func noopSanityChecker(*flow.Header) error { return nil }
