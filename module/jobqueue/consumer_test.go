package jobqueue

import "testing"

// 1. initially state
//    [0]
// [] => 										[0#]
// [+1] => 									[0#, 1!]
// [+1, 1*] => 							[0#, 1#]
// [+1, +2, 1*, 2*] => 			[0#, 1#, 2#]
// [+1, +2, +3, +4] => 			[0#, 1!, 2!, 3!, 4]
// [+1, +2, +3, +4, +5] => 	[0#, 1!, 2!, 3!, 4, 5]
// [+1, +2, +3, +4, 3*] => 	[0#, 1!, 2!, 3*, 4!]
// [+1, +2, +3, +4, 3*, 2*] => 			[0#, 1!, 2*, 3*, 4!]
// [+1, +2, +3, +4, 3*, 2*, +5] =>	[0#, 1!, 2*, 3*, 4!, 5!]
// [+1, +2, +3, +4, 3*, 2*, +5, +6] =>	[0#, 1!, 2*, 3*, 4!, 5!, 6]
// [+1, +2, +3, +4, 3*, 2*, +5, 1*] => [0#, 1#, 2#, 3#, 4!, 5!]
// [+1, +2, +3, +4, 3*, 2*, +5, 1*, +6, +7, 6*], restart => [0#, 1#, 2#, 3#, 4!, 5!, 6*, 7!]
// [+1, +2, +3, ... +12, +13, +14, 1*, 2*, 3*, 5*, 6*, ...12*] => [1#, 2#, 3#, 4!, 5*, 6*, ... 12*, 13, 14]

// TestNonNextIsDoneNoMoreProcessable [0#, 1!, 2*, 3*, 4!]
// TestNoNextIsDoneHasMoreProcessable [0#, 1!, 2*, 3*, 4!, 5!]
// TestFastforward										[0#, 1#, 2#, 3#, 4!, 5!]
// TestCrashResume										[0#, 1#, 2#, 3#, 4!, 5!, 6*, 7!]
// when maxProcessing is 3, there are 4 jobs, only the first 3 will be processed
//
// Test
// 3. when maxProcessing is 3, after the first 3 will be processed,
//    and the 3rd one is finshed, it will process the 4th
// 3. when maxProcessing is 3, after the 2nd and 3rd are processed, no will be processed,
// 4. [1*][2*][3#][4*]
func TestConsumer(t *testing.T) {
}
