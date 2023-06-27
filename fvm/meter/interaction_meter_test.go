package meter

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
)

/*
*  When merging meter interaction limits for a register from meter B to meter A
*  - `RA[k]`  is the **R**ead of register with **k**ey in meter **A**
*  - `WB[k]`  is the **W**rite of register with **k**ey in meter **B**
*  The following rules apply:
*
*  1. `RB[k] != nil && WB[k] == nil`:
*    1. `RA[k] ==nil && WA[k] == nil` -> do: `RA[k] = RB[k]`
*    2. `RA[k] != nil && WA[k] == nil` -> `RA[k]` must be equal to `RB[k]` as B is reading the same register as A did. Nothing to do.
*    3. `RA[k] == nil && WA[k] != nil`-> when register k is read in B the value is taken from the changed value that A metered as a write so no storage read happened in B, `RB[k]` must be equal to `WA[k]`. Nothing to do.
*    4. `RA[k] != nil && WA[k] != nil` -> similar to 1.3. no storage read happened in B as the changed register was read from A.  `WA[k]` must be equal to `RB[k]`. Nothing to do
*
*  2.  `RB[k] == nil && WB[k] != nil`:
*    1. `RA[k] ==nil && WA[k] == nil` -> do: `WA[k] = WB[k]`
*    2. `RA[k] != nil && WA[k] == nil` -> do: `WA[k] = WB[k]`
*    3. `RA[k] == nil && WA[k] != nil` -> the write in B is latest so that one should be used. do: `WA[k] = WB[k]`
*    4. `RA[k] != nil && WA[k] != nil` -> the write in B is latest so that one should be used. do: `WA[k] = WB[k]`
*
*  3. `RB[k] != nil && WB[k] != nil`:
*    1. `RA[k] ==nil && WA[k] == nil` -> do: `WA[k] = WB[k]` and `RA[k] = RB[k]`
*    2. `RA[k] != nil && WA[k] == nil` -> `RA[k]` must be equal to `RB[k]` as B is reading the same register as A did. do: `WA[k] = WB[k]`
*    3. `RA[k] == nil && WA[k] != nil` -> B read should be equal to A write, because the value of the register was taken from the changed value in A, `RB[k]` must be equal to `WA[k]`. do:  `WA[k] = WB[k]`
*    4. `RA[k] != nil && WA[k] != nil` -> similar to 3.3. do:  `WA[k] = WB[k]`
*
*  We shouldn't do error checking in merging, so just assume that cases that shouldn't happen don't happen. The checks for those should be, and are, elsewhere.
*
*  in short this means that when merging we should do the following:
*  - take reads from the parent unless the parents write and read is nil
*  - take writes from the child unless nil
 */
func TestInteractionMeter_Merge(t *testing.T) {
	key := flow.RegisterID{
		Owner: "owner",
		Key:   "key",
	}

	value1 := []byte{1, 2, 3}
	value1Size := getStorageKeyValueSize(key, value1)
	value2 := []byte{4, 5, 6, 7}
	value2Size := getStorageKeyValueSize(key, value2)
	value3 := []byte{8, 9, 10, 11, 12}
	value3Size := getStorageKeyValueSize(key, value3)
	value4 := []byte{8, 9, 10, 11, 12, 13}
	value4Size := getStorageKeyValueSize(key, value4)

	type testCase struct {
		Descripiton string

		ParentReads flow.RegisterValue
		ChildReads  flow.RegisterValue

		ParentWrites flow.RegisterValue
		ChildWrites  flow.RegisterValue

		TotalReadShouldBe    uint64
		TotalWrittenShouldBe uint64
	}

	cases := []testCase{
		{
			Descripiton: "no interaction",

			ParentReads:          nil,
			ParentWrites:         nil,
			ChildReads:           nil,
			ChildWrites:          nil,
			TotalReadShouldBe:    0,
			TotalWrittenShouldBe: 0,
		},
	}

	desc := "child Reads, "
	cases = append(cases,
		testCase{
			Descripiton: desc + "parent Nothing",

			ParentReads:          nil,
			ParentWrites:         nil,
			ChildReads:           value1,
			ChildWrites:          nil,
			TotalReadShouldBe:    value1Size,
			TotalWrittenShouldBe: 0,
		},
		testCase{
			Descripiton: desc + "parent Reads",

			ParentReads:          value1,
			ParentWrites:         nil,
			ChildReads:           value2,
			ChildWrites:          nil,
			TotalReadShouldBe:    value1Size,
			TotalWrittenShouldBe: 0,
		},
		testCase{
			Descripiton: desc + "parent Writes",

			ParentReads:          nil,
			ParentWrites:         value1,
			ChildReads:           value2,
			ChildWrites:          nil,
			TotalReadShouldBe:    0,
			TotalWrittenShouldBe: value1Size,
		},
		testCase{
			Descripiton: desc + "parent Reads and Writes",

			ParentReads:          value1,
			ParentWrites:         value2,
			ChildReads:           value3,
			ChildWrites:          nil,
			TotalReadShouldBe:    value1Size,
			TotalWrittenShouldBe: value2Size,
		},
	)

	desc = "child Writes, "
	cases = append(cases,
		testCase{
			Descripiton: desc + "parent Nothing",

			ParentReads:          nil,
			ParentWrites:         nil,
			ChildReads:           nil,
			ChildWrites:          value1,
			TotalReadShouldBe:    0,
			TotalWrittenShouldBe: value1Size,
		},
		testCase{
			Descripiton: desc + "parent Reads",

			ParentReads:          value1,
			ParentWrites:         nil,
			ChildReads:           nil,
			ChildWrites:          value2,
			TotalReadShouldBe:    value1Size,
			TotalWrittenShouldBe: value2Size,
		},
		testCase{
			Descripiton: desc + "parent Writes",

			ParentReads:          nil,
			ParentWrites:         value1,
			ChildReads:           nil,
			ChildWrites:          value2,
			TotalReadShouldBe:    0,
			TotalWrittenShouldBe: value2Size,
		},
		testCase{
			Descripiton: desc + "parent Reads and Writes",

			ParentReads:          value1,
			ParentWrites:         value2,
			ChildReads:           nil,
			ChildWrites:          value3,
			TotalReadShouldBe:    value1Size,
			TotalWrittenShouldBe: value3Size,
		},
	)

	desc = "child Reads and Writes, "
	cases = append(cases,
		testCase{
			Descripiton: desc + "parent Nothing",

			ParentReads:          nil,
			ParentWrites:         nil,
			ChildReads:           value1,
			ChildWrites:          value2,
			TotalReadShouldBe:    value1Size,
			TotalWrittenShouldBe: value2Size,
		},
		testCase{
			Descripiton: desc + "parent Reads",

			ParentReads:          value1,
			ParentWrites:         nil,
			ChildReads:           value2,
			ChildWrites:          value3,
			TotalReadShouldBe:    value1Size,
			TotalWrittenShouldBe: value3Size,
		},
		testCase{
			Descripiton: desc + "parent Writes",

			ParentReads:          nil,
			ParentWrites:         value1,
			ChildReads:           value2, // this is a read from the parent
			ChildWrites:          value3,
			TotalReadShouldBe:    0,
			TotalWrittenShouldBe: value3Size,
		},
		testCase{
			Descripiton: desc + "parent Reads and Writes",

			ParentReads:          value1,
			ParentWrites:         value2,
			ChildReads:           value3, // this is a read from the parent
			ChildWrites:          value4,
			TotalReadShouldBe:    value1Size,
			TotalWrittenShouldBe: value4Size,
		},
	)

	for i, c := range cases {
		t.Run(fmt.Sprintf("case %d: %s", i, c.Descripiton), func(t *testing.T) {
			parentMeter := NewInteractionMeter(DefaultInteractionMeterParameters())
			childMeter := NewInteractionMeter(DefaultInteractionMeterParameters())

			var err error
			if c.ParentReads != nil {
				err = parentMeter.MeterStorageRead(key, c.ParentReads, false)
				require.NoError(t, err)
			}

			if c.ChildReads != nil {
				err = childMeter.MeterStorageRead(key, c.ChildReads, false)
				require.NoError(t, err)
			}

			if c.ParentWrites != nil {
				err = parentMeter.MeterStorageWrite(key, c.ParentWrites, false)
				require.NoError(t, err)
			}

			if c.ChildWrites != nil {
				err = childMeter.MeterStorageWrite(key, c.ChildWrites, false)
				require.NoError(t, err)
			}

			parentMeter.Merge(childMeter)

			require.Equal(t, c.TotalReadShouldBe, parentMeter.TotalBytesReadFromStorage())
			require.Equal(t, c.TotalWrittenShouldBe, parentMeter.TotalBytesWrittenToStorage())
		})
	}

}
