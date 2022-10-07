package wal

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/onflow/flow-go/ledger/complete/mtrie/flattener"
	"github.com/rs/zerolog/log"
)

func CompareV5(f1 *os.File, f2 *os.File) error {
	nodeCount1, trieCount1, checksum1, err := readTopTriesFooter(f1)
	if err != nil {
		return fmt.Errorf("could not read node count for file 1:%w", err)
	}

	nodeCount2, trieCount2, checksum2, err := readTopTriesFooter(f2)
	if err != nil {
		return fmt.Errorf("could not read node count for file 2:%w", err)
	}

	if nodeCount1 != nodeCount2 {
		return fmt.Errorf("node count different %v != %v", nodeCount1, nodeCount2)
	}

	if trieCount1 != trieCount2 {
		return fmt.Errorf("trie count different %v != %v", trieCount1, trieCount2)
	}

	if checksum1 != checksum2 {
		return fmt.Errorf("checksum different %v != %v", checksum1, checksum2)
	}

	_, err = f1.Seek(0, io.SeekStart)
	if err != nil {
		return fmt.Errorf("cannot seek to start of file 1: %w", err)
	}

	_, err = f2.Seek(0, io.SeekStart)
	if err != nil {
		return fmt.Errorf("cannot seek to start of file 2: %w", err)
	}

	err = validateFileHeader(MagicBytesCheckpointHeader, VersionV5, f1)
	if err != nil {
		return fmt.Errorf("could not read version 1: %w", err)
	}

	err = validateFileHeader(MagicBytesCheckpointHeader, VersionV5, f2)
	if err != nil {
		return fmt.Errorf("could not read version 2: %w", err)
	}

	scratch := make([]byte, 1024*4) // must not be less than 1024

	reader1 := bufio.NewReaderSize(f1, defaultBufioReadSize)
	reader2 := bufio.NewReaderSize(f2, defaultBufioReadSize)

	log.Info().Msgf("comparing %v nodes", nodeCount1)

	logging := logProgress(fmt.Sprintf("compare %v nodes", nodeCount1), int(nodeCount1), &log.Logger)
	for i := 0; i < int(nodeCount1); i++ {
		logging(uint64(i))
		isLeaf1, height1, lchildIndex1, rchildIndex1, path1, payload1, nodeHash1, err := flattener.DecodeNodeData(reader1, scratch)
		if err != nil {
			return fmt.Errorf("can not find node1 %w", err)
		}
		isLeaf2, height2, lchildIndex2, rchildIndex2, path2, payload2, nodeHash2, err := flattener.DecodeNodeData(reader2, scratch)
		if err != nil {
			return fmt.Errorf("can not find node2 %w", err)
		}

		if isLeaf1 != isLeaf2 ||
			height1 != height2 ||
			lchildIndex1 != lchildIndex2 ||
			rchildIndex1 != rchildIndex2 ||
			path1 != path2 ||
			!payload1.Equals(payload2) ||
			nodeHash1 != nodeHash2 {
			return fmt.Errorf(
				"different node, isLeaf: (%v,%v), height: (%v,%v), lchildIndex: (%v,%v), rchildIndex: (%v,%v), "+
					"path: (%x,%x), payload: (%x,%x), nodeHash: (%x,%x)",
				isLeaf1, isLeaf2,
				height1, height2,
				lchildIndex1, lchildIndex2,
				rchildIndex2, rchildIndex2,
				path1, path2,
				payload1, payload2,
				nodeHash1, nodeHash2,
			)
		}
	}

	log.Info().Msgf("comparing %v tries", trieCount1)

	for i := 0; i < int(trieCount1); i++ {
		rootIndex1, regCount1, regSize1, rootHash1, err := flattener.DecodeTrieData(reader1, scratch)
		if err != nil {
			return fmt.Errorf("could not read trie1: %w", err)
		}

		rootIndex2, regCount2, regSize2, rootHash2, err := flattener.DecodeTrieData(reader2, scratch)
		if err != nil {
			return fmt.Errorf("could not read trie2: %w", err)
		}

		if rootIndex1 != rootIndex2 ||
			regCount1 != regCount2 ||
			regSize1 != regSize2 ||
			rootHash1 != rootHash2 {
			return fmt.Errorf("different trie, rootIndex: (%v, %v), regCount: (%v, %v), regSize: (%v, %v), rootHash: (%x, %x)",
				rootIndex1, rootIndex2,
				regCount1, regCount2,
				regSize1, regSize2,
				rootHash1, rootHash2)
		}
	}

	log.Info().Msg("comparing footer")

	// read footer and discard, since we only care about checksum
	_, err = io.ReadFull(reader1, scratch[:encNodeCountSize+encTrieCountSize])
	if err != nil {
		return fmt.Errorf("fail to read node count 1: %w", err)
	}
	_, err = io.ReadFull(reader2, scratch[:encNodeCountSize+encTrieCountSize])
	if err != nil {
		return fmt.Errorf("fail to read node count 2: %w", err)
	}

	sum1, err := readCRC32Sum(reader1)
	if err != nil {
		return fmt.Errorf("can not read sum1: %w", err)
	}

	sum2, err := readCRC32Sum(reader2)
	if err != nil {
		return fmt.Errorf("can not read sum2: %w", err)
	}

	if sum1 != sum2 {
		return fmt.Errorf("different sum %v != %v", sum1, sum2)
	}

	log.Info().Msg("checkpoint1 and checkpoint2 are consistent")

	return nil
}
