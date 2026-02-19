package reporters

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// TODO janezp: we should be able to switch the report writer to write to a database.
type ReportWriterFactory interface {
	ReportWriter(dataNamespace string) ReportWriter
}

type ReportFileWriterFactory struct {
	fileSuffix int32
	outputDir  string
	log        zerolog.Logger
	format     ReportFormat
}

type ReportFormat uint8

const (
	// ReportFormatJSONArray represents format encoded as JSON array at the top level.
	ReportFormatJSONArray ReportFormat = iota

	// ReportFormatJSONL represents format encoded as JSONL.
	// ReportFormatJSONL should be used when report might be large enough to
	// crash tools like jq (if JSON array is used instead of JSONL).
	ReportFormatJSONL

	ReportFormatCSV
)

func NewReportFileWriterFactory(outputDir string, log zerolog.Logger) *ReportFileWriterFactory {
	return NewReportFileWriterFactoryWithFormat(outputDir, log, ReportFormatJSONArray)
}

func NewReportFileWriterFactoryWithFormat(outputDir string, log zerolog.Logger, format ReportFormat) *ReportFileWriterFactory {
	return &ReportFileWriterFactory{
		fileSuffix: int32(time.Now().Unix()),
		outputDir:  outputDir,
		log:        log,
		format:     format,
	}
}

func (r *ReportFileWriterFactory) Filename(dataNamespace string) string {
	switch r.format {
	case ReportFormatJSONArray:
		return path.Join(r.outputDir, fmt.Sprintf("%s_%d.json", dataNamespace, r.fileSuffix))

	case ReportFormatJSONL:
		return path.Join(r.outputDir, fmt.Sprintf("%s_%d.jsonl", dataNamespace, r.fileSuffix))

	case ReportFormatCSV:
		return path.Join(r.outputDir, fmt.Sprintf("%s_%d.csv", dataNamespace, r.fileSuffix))

	default:
		panic(fmt.Sprintf("unrecognized report format: %d", r.format))
	}
}

func (r *ReportFileWriterFactory) ReportWriter(dataNamespace string) ReportWriter {
	fn := r.Filename(dataNamespace)

	return NewReportFileWriter(fn, r.log, r.format)
}

var _ ReportWriterFactory = &ReportFileWriterFactory{}

const reportFileWriteBufferSize = 100

func NewReportFileWriter(fileName string, log zerolog.Logger, format ReportFormat) ReportWriter {
	switch format {
	case ReportFormatCSV:
		return NewCSVReportFileWriter(fileName, log)
	case ReportFormatJSONArray, ReportFormatJSONL:
		return NewJSONReportFileWriter(fileName, log, format)
	default:
		panic(fmt.Sprintf("report format %d not supported", format))
	}
}

// ReportWriter writes data from reports
type ReportWriter interface {
	Write(dataPoint any)
	Close()
}

// ReportNilWriter

// ReportNilWriter does nothing. Can be used as the final fallback writer
type ReportNilWriter struct {
}

var _ ReportWriter = &ReportNilWriter{}

func (r ReportNilWriter) Write(_ any) {
}

func (r ReportNilWriter) Close() {
}

// JSONReportFileWriter

type JSONReportFileWriter struct {
	f          *os.File
	fileName   string
	wg         *sync.WaitGroup
	writeChan  chan any
	writer     *bufio.Writer
	log        zerolog.Logger
	format     ReportFormat
	faulty     bool
	firstWrite bool
}

var _ ReportWriter = &JSONReportFileWriter{}

func NewJSONReportFileWriter(fileName string, log zerolog.Logger, format ReportFormat) ReportWriter {
	f, err := os.Create(fileName)
	if err != nil {
		log.Warn().Err(err).Msg("Error creating ReportFileWriter, defaulting to ReportNilWriter")
		return ReportNilWriter{}
	}

	writer := bufio.NewWriter(f)

	if format == ReportFormatJSONArray {
		// Open top-level JSON array
		_, err = writer.WriteRune('[')

		if err != nil {
			log.Warn().Err(err).Msg("Error opening json array")
			// time to clean up
			err = writer.Flush()
			if err != nil {
				log.Error().Err(err).Msg("Error closing flushing writer")
				panic(err)
			}

			err = f.Close()
			if err != nil {
				log.Error().Err(err).Msg("Error closing report file")
				panic(err)
			}
			return ReportNilWriter{}
		}
	}

	fw := &JSONReportFileWriter{
		f:          f,
		fileName:   fileName,
		writer:     writer,
		log:        log,
		firstWrite: true,
		writeChan:  make(chan any, reportFileWriteBufferSize),
		wg:         &sync.WaitGroup{},
		format:     format,
	}

	fw.wg.Add(1)
	go func() {

		for d := range fw.writeChan {
			fw.write(d)
		}
		fw.wg.Done()
	}()

	return fw
}

func (r *JSONReportFileWriter) Write(dataPoint any) {
	r.writeChan <- dataPoint
}

func (r *JSONReportFileWriter) write(dataPoint any) {
	if r.faulty {
		return
	}
	tc, err := json.Marshal(dataPoint)
	if err != nil {
		r.log.Warn().Err(err).Msg("Error converting data point to json")
		r.faulty = true
	}

	if !r.firstWrite {
		switch r.format {
		case ReportFormatJSONArray:
			// delimit the json records with commas
			_, err = r.writer.WriteRune(',')
			if err != nil {
				r.log.Warn().Err(err).Msg("Error writing JSON array delimiter to file")
				r.faulty = true
			}

		case ReportFormatJSONL:
			// delimit the json records with line break
			_, err = r.writer.WriteRune('\n')
			if err != nil {
				r.log.Warn().Err(err).Msg("Error Writing JSONL delimiter to file")
				r.faulty = true
			}
		}
	} else {
		r.firstWrite = false
	}

	_, err = r.writer.Write(tc)
	if err != nil {
		r.log.Warn().Err(err).Msg("Error Writing json to file")
		r.faulty = true
	}
}

func (r *JSONReportFileWriter) Close() {
	close(r.writeChan)
	r.wg.Wait()

	if r.format == ReportFormatJSONArray {
		// Close top-level json array
		_, err := r.writer.WriteRune(']')
		if err != nil {
			r.log.Warn().Err(err).Msg("Error finishing json array")
			// nothing to do, we will be closing the file now
		}
	}

	err := r.writer.Flush()
	if err != nil {
		r.log.Error().Err(err).Msg("Error closing flushing writer")
		panic(err)
	}

	err = r.f.Close()
	if err != nil {
		r.log.Error().Err(err).Msg("Error closing report file")
		panic(err)
	}

	r.log.Info().Str("filename", r.fileName).Msg("Created report file")
}

// CSVReportFileWriter

type CSVReportFileWriter struct {
	f         *os.File
	fileName  string
	wg        *sync.WaitGroup
	writeChan chan []string
	writer    *csv.Writer
	log       zerolog.Logger
	faulty    bool
}

var _ ReportWriter = &CSVReportFileWriter{}

func NewCSVReportFileWriter(fileName string, log zerolog.Logger) ReportWriter {
	f, err := os.Create(fileName)
	if err != nil {
		log.Warn().Err(err).Msg("Error creating ReportFileWriter, defaulting to ReportNilWriter")
		return ReportNilWriter{}
	}

	writer := csv.NewWriter(f)

	fw := &CSVReportFileWriter{
		f:         f,
		fileName:  fileName,
		writer:    writer,
		log:       log,
		writeChan: make(chan []string, reportFileWriteBufferSize),
		wg:        &sync.WaitGroup{},
	}

	fw.wg.Add(1)
	go func() {

		for d := range fw.writeChan {
			fw.write(d)
		}
		fw.wg.Done()
	}()

	return fw
}

func (r *CSVReportFileWriter) Write(dataPoint any) {
	record, ok := dataPoint.([]string)
	if !ok {
		r.log.Warn().Msgf("cannot write %T to csv, skip this record", dataPoint)
		return
	}

	r.writeChan <- record
}

func (r *CSVReportFileWriter) write(record []string) {
	if r.faulty {
		return
	}
	err := r.writer.Write(record)
	if err != nil {
		r.log.Warn().Err(err).Msg("error writing to csv file")
		r.faulty = true
	}
}

func (r *CSVReportFileWriter) Close() {
	close(r.writeChan)
	r.wg.Wait()

	r.writer.Flush()

	err := r.f.Close()
	if err != nil {
		r.log.Error().Err(err).Msg("Error closing report file")
		panic(err)
	}

	r.log.Info().Str("filename", r.fileName).Msg("Created report file")
}
