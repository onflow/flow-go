package reporters

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// TODO janezp: we should be able to swithch the report writer to write to a database.
type ReportWriterFactory interface {
	ReportWriter(dataNamespace string) ReportWriter
}

type ReportFileWriterFactory struct {
	fileSuffix int32
	outputDir  string
	log        zerolog.Logger
}

func NewReportFileWriterFactory(outputDir string, log zerolog.Logger) *ReportFileWriterFactory {
	return &ReportFileWriterFactory{
		fileSuffix: int32(time.Now().Unix()),
		outputDir:  outputDir,
		log:        log,
	}
}

func (r *ReportFileWriterFactory) Filename(dataNamespace string) string {
	return path.Join(r.outputDir, fmt.Sprintf("%s_%d.json", dataNamespace, r.fileSuffix))
}

func (r *ReportFileWriterFactory) ReportWriter(dataNamespace string) ReportWriter {
	fn := r.Filename(dataNamespace)

	return NewReportFileWriter(fn, r.log)
}

var _ ReportWriterFactory = &ReportFileWriterFactory{}

// ReportWriter writes data from reports
type ReportWriter interface {
	Write(dataPoint interface{})
	Close()
}

// ReportNilWriter does nothing. Can be used as the final fallback writer
type ReportNilWriter struct {
}

var _ ReportWriter = &ReportNilWriter{}

func (r ReportNilWriter) Write(_ interface{}) {
}

func (r ReportNilWriter) Close() {
}

var _ ReportWriter = &ReportFileWriter{}

type ReportFileWriter struct {
	f          *os.File
	fileName   string
	wg         *sync.WaitGroup
	writeChan  chan interface{}
	writer     *bufio.Writer
	log        zerolog.Logger
	faulty     bool
	firstWrite bool
}

const reportFileWriteBufferSize = 100

func NewReportFileWriter(fileName string, log zerolog.Logger) ReportWriter {
	f, err := os.Create(fileName)
	if err != nil {
		log.Warn().Err(err).Msg("Error creating ReportFileWriter, defaulting to ReportNilWriter")
		return ReportNilWriter{}
	}

	writer := bufio.NewWriter(f)

	// open a json array
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

	fw := &ReportFileWriter{
		f:          f,
		fileName:   fileName,
		writer:     writer,
		log:        log,
		firstWrite: true,
		writeChan:  make(chan interface{}, reportFileWriteBufferSize),
		wg:         &sync.WaitGroup{},
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

func (r *ReportFileWriter) Write(dataPoint interface{}) {
	r.writeChan <- dataPoint
}

func (r *ReportFileWriter) write(dataPoint interface{}) {
	if r.faulty {
		return
	}
	tc, err := json.Marshal(dataPoint)
	if err != nil {
		r.log.Warn().Err(err).Msg("Error converting data point to json")
		r.faulty = true
	}

	// delimit the json records with commas
	if !r.firstWrite {
		_, err = r.writer.WriteRune(',')
		if err != nil {
			r.log.Warn().Err(err).Msg("Error Writing json to file")
			r.faulty = true
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

func (r *ReportFileWriter) Close() {
	close(r.writeChan)
	r.wg.Wait()

	_, err := r.writer.WriteRune(']')
	if err != nil {
		r.log.Warn().Err(err).Msg("Error finishing json array")
		// nothing to do, we will be closing the file now
	}
	err = r.writer.Flush()
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
