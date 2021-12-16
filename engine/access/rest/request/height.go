package request

import (
	"fmt"
	"math"
	"strconv"
)

const sealed = "sealed"
const final = "final"

const SealedHeight = math.MaxUint64 - 1
const FinalHeight = math.MaxUint64 - 2

type Height uint64

func (h *Height) Parse(raw string) error {
	if raw == "" { // allow empty
		return nil
	}

	if raw == sealed {
		*h = SealedHeight
	}
	if raw == final {
		*h = FinalHeight
	}

	height, err := strconv.ParseUint(raw, 0, 64)
	if err != nil {
		return fmt.Errorf("invalid height format")
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
	heights := make([]Height, len(raw))
	for i, r := range raw {
		err := height.Parse(r)
		if err != nil {
			return err
		}

		heights[i] = height
	}
	return nil
}

func (h Heights) Flow() []uint64 {
	heights := make([]uint64, len(h))
	for i, he := range h {
		heights[i] = he.Flow()
	}
	return heights
}
