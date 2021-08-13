package badger

import "testing"

// 1. should be able to read after store
// 2. should be able to read the latest index after store
// 3. should return false if a duplicate chunk is stored
// 4. should return true if a new chunk is stored
// 5. should return an increased index when a chunk is stored
// 6. storing 100 chunks concurrent should return last index as 100
// 7. should not be able to read with wrong index
// 8. should return init index after init
// 9. storing chunk and updating the latest index should be atomic
func TestStoreAndRead(t *testing.T) {
	// TODO
}
