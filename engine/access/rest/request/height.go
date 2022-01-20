package request

import (
	"fmt"
	"math"
	"strconv"
)

const sealed = "sealed"
const final = "final"

// Special height values
const SealedHeight uint64 = math.MaxUint64 - 1
const FinalHeight uint64 = math.MaxUint64 - 2
const EmptyHeight uint64 = math.MaxUint64 - 3

type Height uint64

func (h *Height) Parse(raw string) error {
	if raw == "" { // allow empty
		*h = Height(EmptyHeight)
		return nil
	}

	if raw == sealed {
		*h = Height(SealedHeight)
		return nil
	}
	if raw == final {
		*h = Height(FinalHeight)
		return nil
	}

	height, err := strconv.ParseUint(raw, 0, 64)
	if err != nil {
		return fmt.Errorf("invalid height format")
	}

	if height >= EmptyHeight {
		return fmt.Errorf("invalid height value")
	}

	*h = Height(height)
	return nil
}

func (h Height) Flow() uint64 {
	return uint64(h)
}

type Heights []Height

func (h *Heights) Parse(raw []string) error {
	var height Height
	heights := make([]Height, 0)
	for _, r := range raw {
		err := height.Parse(r)
		if err != nil {
			return err
		}
		// don't include empty heights
		if height == Height(EmptyHeight) {
			continue
		}

		heights = append(heights, height)
	}

	*h = heights
	return nil
}

func (h Heights) Flow() []uint64 {
	heights := make([]uint64, len(h))
	for i, he := range h {
		heights[i] = he.Flow()
	}
	return heights
}
