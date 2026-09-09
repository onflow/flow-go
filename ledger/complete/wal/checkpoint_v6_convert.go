package wal

import (
	"fmt"
	"os"

	prometheusWAL "github.com/onflow/wal/wal"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/bootstrap"
)

// leaf-hash presence flag values in the V7 (payloadless) leaf encoding. They
// mirror the (unexported) leafHashAbsent / leafHashPresent constants in
// ledger/complete/payloadless/flattener.go; they are duplicated here because the
// streaming converter operates on the raw byte stream rather than through the
// payloadless flattener.
const (
	leafHashAbsentFlag  = byte(0)
	leafHashPresentFlag = byte(1)
)

// ConvertCheckpointV7ToV6 reconstructs a full V6 checkpoint from a V7
// (payloadless) checkpoint by re-sourcing every leaf's payload from a previous
// full V6 checkpoint plus the WAL segments written between that previous
// checkpoint and the V7 checkpoint.
//
// Rationale: a V7 checkpoint stores only a leaf hash per register, not the
// payload, so it cannot be turned back into a V6 checkpoint on its own. However,
// every payload referenced by the V7 checkpoint must exist either in the previous
// full checkpoint (if the register was not updated since) or in one of the WAL
// segments written since (if it was). By computing HashLeaf(path, value) for each
// candidate payload from those two sources and matching it against the leaf hash
// stored in the V7 checkpoint, the original payload is recovered.
//
// Inputs:
//   - (v7Dir, v7File): the V7 checkpoint header file to convert, e.g.
//     "checkpoint.00000100.v7". Its number N is parsed from the filename.
//   - execDir: the standard ledger WAL directory holding both the previous V6
//     checkpoint part files and the numbered WAL segment files.
//   - prevCheckpointNum: the previous full V6 checkpoint number M to source
//     unchanged payloads from. If negative, it is auto-discovered as the latest
//     V6 checkpoint in execDir with number strictly less than N. If no such
//     numbered checkpoint exists, it falls back to the V6 root checkpoint, in
//     which case the full WAL range [0, N] is replayed.
//   - walFrom, walTo: the inclusive WAL segment range to source updated payloads
//     from. If negative, they default to (M+1, N] — i.e. all updates applied
//     after the previous checkpoint up to and including the V7 checkpoint state.
//   - (outputDir, outputFile): where to write the reconstructed V6 checkpoint.
//
// Memory: the checkpoint is processed one subtrie partition (first path nibble)
// at a time. For partition i, only that partition's payloads are held in memory
// while its V6 subtrie part file is written, then released before the next
// partition. nWorker partitions are processed concurrently (valid range
// [1, subtrieCount]), trading peak memory for speed.
//
// Limitation: the per-trie regSize (AllocatedRegSize) field is metrics-only and
// is dropped by the V7 format; it is NOT reconstructed and is written as 0 in
// every trie root record. A warning is logged. This does not affect trie root
// hashes or any consensus-critical state; it only affects the LatestTrieRegSize
// metric until the node rebuilds tries.
//
// The output filename must NOT carry the V7 suffix and no output part file may
// already exist; otherwise the call is rejected. On any failure, partially
// written output files are removed.
//
// No error returns are expected during normal operation; all error returns
// indicate malformed input, missing source data, an unmatched leaf hash, a
// clobbering output, or an IO failure.
func ConvertCheckpointV7ToV6(
	v7Dir string,
	v7File string,
	execDir string,
	prevCheckpointNum int,
	walFrom int,
	walTo int,
	outputDir string,
	outputFile string,
	logger zerolog.Logger,
	nWorker uint,
) error {
	err := convertCheckpointV7ToV6(
		v7Dir, v7File, execDir, prevCheckpointNum, walFrom, walTo, outputDir, outputFile, logger, nWorker)
	if err != nil {
		cleanupErr := deleteCheckpointFiles(outputDir, outputFile)
		if cleanupErr != nil {
			return fmt.Errorf("fail to cleanup temp file %s, after running into error: %w", cleanupErr, err)
		}
		return err
	}
	return nil
}

func convertCheckpointV7ToV6(
	v7Dir string,
	v7File string,
	execDir string,
	prevCheckpointNum int,
	walFrom int,
	walTo int,
	outputDir string,
	outputFile string,
	logger zerolog.Logger,
	nWorker uint,
) error {
	if nWorker == 0 || nWorker > subtrieCount {
		return fmt.Errorf("invalid nWorker %v, valid range is [1, %v]", nWorker, subtrieCount)
	}

	// The output is a V6 checkpoint, so it must not use the V7 suffix.
	if err := requireV6Filename(outputFile); err != nil {
		return err
	}

	// Validate V7 input exists and read its part-file checksums.
	v7Header := filePathCheckpointHeader(v7Dir, v7File)
	if _, err := os.Stat(v7Header); err != nil {
		return fmt.Errorf("V7 checkpoint header not found at %s: %w", v7Header, err)
	}
	v7SubtrieChecksums, v7TopTrieChecksum, err := readCheckpointHeaderV7(v7Header, logger)
	if err != nil {
		return fmt.Errorf("could not read V7 checkpoint header: %w", err)
	}
	if err := allPartFileExist(v7Dir, v7File, len(v7SubtrieChecksums)); err != nil {
		return fmt.Errorf("V7 part files incomplete for %s/%s: %w", v7Dir, v7File, err)
	}

	// Determine the V7 checkpoint number N from its filename.
	v7Info, ok := parseCheckpointFilename(v7File)
	if !ok || v7Info.Version != VersionV7 {
		return fmt.Errorf("could not parse V7 checkpoint number from filename %q", v7File)
	}
	n := v7Info.Number

	// Resolve the previous full V6 checkpoint (M) to source unchanged payloads.
	// prevNum is -1 when the resolved source is the V6 root checkpoint, which
	// seeds WAL segment 0 (see resolveWALRange).
	prevNum, prevFile, err := resolvePrevCheckpoint(execDir, n, prevCheckpointNum)
	if err != nil {
		return err
	}
	prevHeaderPath := filePathCheckpointHeader(execDir, prevFile)
	if _, err := os.Stat(prevHeaderPath); err != nil {
		return fmt.Errorf("previous V6 checkpoint header not found at %s: %w", prevHeaderPath, err)
	}
	prevSubtrieChecksums, prevTopTrieChecksum, err := readCheckpointHeader(prevHeaderPath, logger)
	if err != nil {
		return fmt.Errorf("could not read previous V6 checkpoint header: %w", err)
	}
	if err := allPartFileExist(execDir, prevFile, len(prevSubtrieChecksums)); err != nil {
		return fmt.Errorf("previous V6 part files incomplete for %s/%s: %w", execDir, prevFile, err)
	}

	// Resolve and validate the WAL segment range (M, N] to source updated payloads.
	from, to, err := resolveWALRange(execDir, prevNum, n, walFrom, walTo)
	if err != nil {
		return err
	}

	// Validate V6 output is not present (any of the part files).
	v6Existing, err := findCheckpointPartFiles(outputDir, outputFile)
	if err != nil {
		return fmt.Errorf("could not check existing V6 output files: %w", err)
	}
	if len(v6Existing) != 0 {
		return fmt.Errorf("V6 output already exists: %v", v6Existing)
	}

	// Remove any leftover temp part files from a previously interrupted conversion.
	if err := removeStaleTempFiles(outputDir, outputFile, logger); err != nil {
		return fmt.Errorf("could not remove stale temp files: %w", err)
	}

	logger.Info().
		Str("v7_dir", v7Dir).
		Str("v7_file", v7File).
		Str("exec_dir", execDir).
		Int("v7_number", n).
		Int("prev_checkpoint", prevNum).
		Int("wal_from", from).
		Int("wal_to", to).
		Str("output_dir", outputDir).
		Str("output_file", outputFile).
		Uint("nworker", nWorker).
		Msg("starting streaming V7→V6 checkpoint conversion")

	src := payloadSource{
		execDir:              execDir,
		prevFile:             prevFile,
		prevSubtrieChecksums: prevSubtrieChecksums,
		prevTopTrieChecksum:  prevTopTrieChecksum,
		walFrom:              from,
		walTo:                to,
	}

	// The top-trie part file may contain leaf nodes for registers that sit above
	// the subtrie split (rare; only when a register is alone in a top-level
	// subtree). Their payloads can belong to any partition, so they are sourced
	// separately. This pre-scan is cheap and is usually empty for dense state.
	topPool, err := buildTopTriePayloadPool(v7Dir, v7File, v7TopTrieChecksum, src, logger)
	if err != nil {
		return fmt.Errorf("could not build top-trie payload pool: %w", err)
	}

	// Convert the 16 subtrie part files concurrently, recomputing each checksum.
	newSubtrieChecksums, err := convertSubTriesV7ToV6Concurrently(
		v7Dir, v7File, outputDir, outputFile, v7SubtrieChecksums, src, logger, nWorker)
	if err != nil {
		return fmt.Errorf("could not convert subtrie files: %w", err)
	}

	// Convert the top-trie part file.
	newTopTrieChecksum, err := convertTopTrieFileV7ToV6(
		v7Dir, v7File, outputDir, outputFile, v7TopTrieChecksum, topPool, logger)
	if err != nil {
		return fmt.Errorf("could not convert top-trie file: %w", err)
	}

	// Write the V6 header referencing the freshly computed checksums.
	if err := storeCheckpointHeader(newSubtrieChecksums, newTopTrieChecksum, outputDir, outputFile, logger); err != nil {
		return fmt.Errorf("could not write V6 checkpoint header: %w", err)
	}

	// Sanity check: the reconstructed V6 root hashes must equal the V7 root hashes
	// (they are carried over verbatim, so this validates the written file is
	// well-formed and consistent).
	if err := verifyRootHashesMatch(v7Dir, v7File, outputDir, outputFile, logger); err != nil {
		return fmt.Errorf("root hash verification failed: %w", err)
	}

	logger.Info().Msg("stream V7→V6 checkpoint conversion complete")
	return nil
}

// payloadSource describes where reconstructed payloads are sourced from: the
// previous full V6 checkpoint and the WAL segment range written since.
type payloadSource struct {
	execDir              string
	prevFile             string
	prevSubtrieChecksums []uint32
	prevTopTrieChecksum  uint32
	walFrom              int
	walTo                int
}

// resolvePrevCheckpoint returns the previous full V6 checkpoint to source
// unchanged payloads from, as both its number and its on-disk filename.
//
// If override is non-negative it is used directly (and must be < n). Otherwise
// the latest numbered V6 checkpoint with number < n in execDir is used. If no
// such numbered checkpoint exists, it falls back to the V6 root checkpoint
// (bootstrap.FilenameWALRootCheckpoint), which is the bootstrap full checkpoint
// seeding WAL segment 0: sourcing from it requires replaying the full WAL range
// [0, n] (resolveWALRange derives walFrom = prevNum+1 = 0 from the returned
// prevNum of -1).
//
// The returned prevNum is the resolved checkpoint number, or -1 when the source
// is the root checkpoint.
//
// No error returns are expected during normal operation.
func resolvePrevCheckpoint(execDir string, n int, override int) (prevNum int, prevFile string, err error) {
	if override >= 0 {
		if override >= n {
			return 0, "", fmt.Errorf("previous checkpoint %d must be less than V7 checkpoint %d", override, n)
		}
		return override, NumberToFilename(override), nil
	}

	nums, _, err := ListV6Checkpoints(execDir)
	if err != nil {
		return 0, "", fmt.Errorf("could not list V6 checkpoints in %s: %w", execDir, err)
	}
	prev := -1
	for _, num := range nums {
		if num < n && num > prev {
			prev = num
		}
	}
	if prev >= 0 {
		return prev, NumberToFilename(prev), nil
	}

	// No numbered V6 checkpoint below n. Fall back to the V6 root checkpoint if
	// present: it is a full checkpoint that, together with the WAL replayed from
	// segment 0, can source every payload.
	hasRoot, err := HasRootCheckpoint(execDir)
	if err != nil {
		return 0, "", fmt.Errorf("could not check for V6 root checkpoint in %s: %w", execDir, err)
	}
	if hasRoot {
		return -1, bootstrap.FilenameWALRootCheckpoint, nil
	}

	return 0, "", fmt.Errorf("no previous V6 checkpoint with number < %d and no V6 root checkpoint found in %s; "+
		"a full checkpoint is required to source unchanged payloads", n, execDir)
}

// resolveWALRange returns the inclusive WAL segment range to replay. When
// overrideFrom / overrideTo are negative they default to (prevNum, n] = [prevNum+1, n].
// The resolved range is validated against the segments available in execDir.
// A returned range with from > to indicates no WAL replay is needed.
//
// No error returns are expected during normal operation.
func resolveWALRange(execDir string, prevNum int, n int, overrideFrom int, overrideTo int) (from int, to int, err error) {
	from = prevNum + 1
	if overrideFrom >= 0 {
		from = overrideFrom
	}
	to = n
	if overrideTo >= 0 {
		to = overrideTo
	}

	if from > to {
		// No updates between the previous checkpoint and the V7 checkpoint.
		return from, to, nil
	}

	first, last, err := prometheusWAL.Segments(execDir)
	if err != nil {
		return 0, 0, fmt.Errorf("could not list WAL segments in %s: %w", execDir, err)
	}
	if first < 0 {
		return 0, 0, fmt.Errorf("no WAL segments found in %s but range [%d, %d] is required", execDir, from, to)
	}
	if from < first || to > last {
		return 0, 0, fmt.Errorf("required WAL segment range [%d, %d] is not fully available; "+
			"segments present are [%d, %d]", from, to, first, last)
	}
	return from, to, nil
}

// requireV6Filename rejects an output filename that is empty or carries the V7
// suffix, since the reconstructed output is a V6 checkpoint.
//
// Expected error returns during normal operation: none.
func requireV6Filename(fileName string) error {
	if fileName == "" {
		return fmt.Errorf("V6 output filename is empty")
	}
	if len(fileName) > len(V7FileSuffix) && fileName[len(fileName)-len(V7FileSuffix):] == V7FileSuffix {
		return fmt.Errorf("V6 output filename %q must not end with %q", fileName, V7FileSuffix)
	}
	return nil
}
