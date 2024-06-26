package reporters

import (
	"bytes"
	"errors"
	"fmt"
	goRuntime "runtime"
	"sync"

	"github.com/onflow/atree"
	"github.com/rs/zerolog"
	"github.com/schollz/progressbar/v3"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

// AtreeReporter iterates payloads and generates payload and atree stats.
type AtreeReporter struct {
	Log zerolog.Logger
	RWF ReportWriterFactory
}

var _ ledger.Reporter = &AtreeReporter{}

func (r *AtreeReporter) Name() string {
	return "Atree Reporter"
}

func (r *AtreeReporter) Report(payloads []ledger.Payload, commit ledger.State) error {
	rwa := r.RWF.ReportWriter("atree_report")
	defer rwa.Close()

	progress := progressbar.Default(int64(len(payloads)), "Processing:")

	workerCount := goRuntime.NumCPU() / 2
	if workerCount == 0 {
		workerCount = 1
	}

	jobs := make(chan *ledger.Payload, workerCount)

	results := make(chan payloadStats, workerCount)
	defer close(results)

	// create multiple workers to process payloads concurrently
	wg := &sync.WaitGroup{}
	wg.Add(workerCount)

	for i := 0; i < workerCount; i++ {
		go func() {
			defer wg.Done()

			var stats payloadStats
			for p := range jobs {
				err := stats.process(p)
				if err != nil {
					k, keyErr := p.Key()
					if keyErr != nil {
						r.Log.Err(keyErr).Msg("failed to get payload key")
					} else {
						r.Log.Err(err).Msgf("failed to process payload %s", k.String())
					}
				}
			}
			results <- stats
		}()
	}

	// produce jobs for workers to process
	for i := 0; i < len(payloads); i++ {
		jobs <- &payloads[i]

		err := progress.Add(1)
		if err != nil {
			panic(fmt.Errorf("progress.Add(1): %w", err))
		}
	}
	close(jobs)

	// wait until all jobs are done
	wg.Wait()

	err := progress.Finish()
	if err != nil {
		panic(fmt.Errorf("progress.Finish(): %w", err))
	}

	// aggregate all payload stats
	var stats payloadStats
	for i := 0; i < workerCount; i++ {
		r := <-results
		stats.add(&r)
	}

	rwa.Write(stats)

	return nil
}

type payloadType uint

const (
	unknownPayloadType payloadType = iota
	fvmPayloadType
	storagePayloadType
	slabPayloadType
)

func getPayloadType(p *ledger.Payload) (payloadType, error) {
	k, err := p.Key()
	if err != nil {
		return unknownPayloadType, err
	}
	if len(k.KeyParts) < 2 {
		return unknownPayloadType, nil
	}

	id := flow.NewRegisterID(
		flow.BytesToAddress(k.KeyParts[0].Value),
		string(k.KeyParts[1].Value))
	if id.IsInternalState() {
		return fvmPayloadType, nil
	}

	if bytes.HasPrefix(k.KeyParts[1].Value, []byte(atree.LedgerBaseStorageSlabPrefix)) {
		return slabPayloadType, nil
	}
	return storagePayloadType, nil
}

type slabPayloadStats struct {
	SlabArrayMetaCount            uint
	SlabArrayDataCount            uint
	SlabMapMetaCount              uint
	SlabMapDataCount              uint
	SlabMapExternalCollisionCount uint
	SlabStorableCount             uint
	SlabMapCollisionGroupCount    uint

	SlabMapCollisionCounts []uint
}

type payloadStats struct {
	FVMPayloadCount     uint
	StoragePayloadCount uint
	SlabPayloadCount    uint
	slabPayloadStats
}

func (s *payloadStats) add(s1 *payloadStats) {
	s.FVMPayloadCount += s1.FVMPayloadCount
	s.StoragePayloadCount += s1.StoragePayloadCount
	s.SlabPayloadCount += s1.SlabPayloadCount
	s.SlabArrayMetaCount += s1.SlabArrayMetaCount
	s.SlabArrayDataCount += s1.SlabArrayDataCount
	s.SlabMapMetaCount += s1.SlabMapMetaCount
	s.SlabMapDataCount += s1.SlabMapDataCount
	s.SlabMapExternalCollisionCount += s1.SlabMapExternalCollisionCount
	s.SlabStorableCount += s1.SlabStorableCount
	s.SlabMapCollisionGroupCount += s1.SlabMapCollisionGroupCount

	if len(s1.SlabMapCollisionCounts) > 0 {
		s.SlabMapCollisionCounts = append(s.SlabMapCollisionCounts, s1.SlabMapCollisionCounts...)
	}
}

func (s *payloadStats) process(p *ledger.Payload) error {
	pt, err := getPayloadType(p)
	if err != nil {
		return err
	}
	switch pt {
	case unknownPayloadType:
		return fmt.Errorf("unknown payload: %s", p.String())
	case fvmPayloadType:
		s.FVMPayloadCount++
	case storagePayloadType:
		s.StoragePayloadCount++
	case slabPayloadType:
		s.SlabPayloadCount++
	}

	if pt != slabPayloadType {
		return nil
	}

	if len(p.Value()) < versionAndFlagSize {
		return errors.New("data is too short")
	}

	flag := p.Value()[flagIndex]

	switch dataType := getSlabType(flag); dataType {
	case slabArray:
		switch arrayDataType := getSlabArrayType(flag); arrayDataType {
		case slabArrayData:
			s.SlabArrayDataCount++

		case slabArrayMeta:
			s.SlabArrayMetaCount++

		default:
			return fmt.Errorf("slab array has invalid flag 0x%x", flag)
		}

	case slabMap:
		switch mapDataType := getSlabMapType(flag); mapDataType {
		case slabMapData:
			_, collisionGroupCount, err := getCollisionGroupCountFromSlabMapData(p.Value())
			if err != nil {
				return err
			}
			if collisionGroupCount > 0 {
				_, inlineCollisionCount, err := getInlineCollisionCountsFromSlabMapData(p.Value())
				if err != nil {
					return err
				}
				if len(inlineCollisionCount) > 0 {
					s.SlabMapCollisionCounts = append(s.SlabMapCollisionCounts, inlineCollisionCount...)
				}
			}
			s.SlabMapCollisionGroupCount += collisionGroupCount
			s.SlabMapDataCount++

		case slabMapCollisionGroup:
			elements, err := parseSlabMapData(p.Value())
			if err != nil {
				return err
			}
			_, rawElements, err := parseRawElements(elements, decMode)
			if err != nil {
				return err
			}
			if len(rawElements) > 0 {
				s.SlabMapCollisionCounts = append(s.SlabMapCollisionCounts, uint(len(rawElements)))
			}
			s.SlabMapExternalCollisionCount++

		case slabMapMeta:
			s.SlabMapMetaCount++

		default:
			return fmt.Errorf("slab map has invalid flag 0x%x", flag)
		}

	case slabStorable:
		s.SlabStorableCount++

	default:
		return fmt.Errorf("slab data has invalid flag 0x%x", flag)
	}

	return nil
}
