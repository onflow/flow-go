package chunking

// import (
// 	"errors"
// 	"math"
// 	"reflect"

// 	"github.com/dapperlabs/flow-go/crypto"
// )

// // SeedPrep prepares a seed for Rand
// func SeedPrep(proof []byte) ([]uint64, error) {
// 	if len(proof)%8 != 0 {
// 		return nil, errors.New("Proof size should be multiple of 8")
// 	}
// 	seed := make([]uint64, 0, len(proof)/8)
// 	for i := 0; i < len(proof); i += 8 {
// 		data := proof[i : i+8]
// 		var ret uint64
// 		for j := uint(0); j < 8; j++ {
// 			ret = (ret << 8) | uint64(data[j])
// 		}
// 		// seed parts can't be zeros
// 		if ret == uint64(0) {
// 			ret = uint64(1)
// 		}
// 		seed = append(seed, ret)
// 	}
// 	return seed, nil
// }

// // FisherYatesShuffle shuffles an slice of a chunks and returns the subset
// func FisherYatesShuffle(seed []uint64, subset int, items []exec.Chunk) []exec.Chunk {
// 	selectedchunks := make([]exec.Chunk, len(items))
// 	copy(selectedchunks, items)
// 	N := len(items)
// 	rand, _ := crypto.NewRand(seed)
// 	// Fisher–Yates shuffle (only continue till n)
// 	for i := 0; i < subset; i++ {
// 		// choose index uniformly in [i, N-1]
// 		r := i + rand.IntN(N-i)
// 		selectedchunks[r], selectedchunks[i] = selectedchunks[i], selectedchunks[r]
// 	}
// 	return selectedchunks[:subset]
// }

// // ChunkSelfSelect provides a way to select a subset of chunks based on verifier's key
// func ChunkSelfSelect(er exec.ExecutionResult, checkRatio float64, sk crypto.PrivateKey) (selectedchunks []exec.Chunk, proof []byte, err error) {
// 	// hasher Hasher
// 	hasher, err := crypto.NewHasher(crypto.SHA3_256)
// 	if err != nil {
// 		return nil, nil, errors.New("failed to load hasher")
// 	}
// 	temp := er.Hash()
// 	proof, err = sk.Sign(temp, hasher)
// 	if err != nil {
// 		return nil, nil, errors.New("failed to generate proof")
// 	}
// 	seed, _ := SeedPrep(proof)
// 	n := int(math.Ceil(float64(len(er.Chunks)) * checkRatio))
// 	selectedChunks := FisherYatesShuffle(seed, n, er.Chunks)
// 	return selectedChunks, proof, nil
// }

// // VerifyChunkSelfSelect verifies ChunkSelfSelect output
// func VerifyChunkSelfSelect(er exec.ExecutionResult, checkRatio float64, pk crypto.PublicKey, selectedchunks []exec.Chunk, proof []byte) (verified bool, err error) {
// 	// check proof
// 	hasher, err := crypto.NewHasher(crypto.SHA3_256)
// 	if err != nil {
// 		return false, errors.New("failed to load hasher")
// 	}
// 	temp := er.Hash()
// 	result, err := pk.Verify(proof, temp, hasher)
// 	if err != nil {
// 		return false, errors.New("failed to verify proof")
// 	}
// 	if !result {
// 		return false, nil
// 	}
// 	// check computation
// 	seed, _ := SeedPrep(proof)
// 	n := int(math.Ceil(float64(len(er.Chunks)) * checkRatio))
// 	expectedSelectedChunks := FisherYatesShuffle(seed, n, er.Chunks)
// 	return reflect.DeepEqual(expectedSelectedChunks, selectedchunks), nil
// }
