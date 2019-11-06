package crypto

// FisherYatesShuffle shuffles a slice of ints with size n using the given seed s and returns a subset m of it
func FisherYatesShuffle(n int, m int, s []uint64) []int {
	items := make([]int, n)
	rand, _ := NewRand(s)
	// Fisher–Yates shuffle (inside-out version)
	for i := 0; i < n-1; i++ {
		j := rand.IntN(i + 1)
		if j != i {
			items[i] = items[j]
		}
		items[j] = i
	}
	return items[:m]
}
