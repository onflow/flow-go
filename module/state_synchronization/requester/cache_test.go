package requester

// import (
// 	"math/rand"
// 	"testing"
// 	"time"

// 	"github.com/stretchr/testify/assert"
// )

// func TestInsert(t *testing.T) {
// 	var heights []uint64
// 	var expected []uint64

// 	for i := uint64(0); i < 100000; i++ {
// 		heights = append(heights, i)
// 		expected = append([]uint64{i}, expected...)
// 	}

// 	rand.Seed(time.Now().UnixNano())

// 	actual := make([]uint64, 0, len(heights))
// 	for len(heights) > 0 {
// 		pos := rand.Intn(len(heights))
// 		actual = insert(actual, heights[pos])
// 		heights = append(heights[:pos], heights[pos+1:]...)
// 	}

// 	assert.Equal(t, expected, actual)
// }
