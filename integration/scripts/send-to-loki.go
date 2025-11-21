package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type LokiStream struct {
	Stream map[string]string `json:"stream"`
	Values [][]string        `json:"values"`
}

type LokiPushRequest struct {
	Streams []LokiStream `json:"streams"`
}

func main() {
	var (
		lokiURL    = flag.String("url", "http://localhost:3100/loki/api/v1/push", "Loki push API URL")
		job        = flag.String("job", "go-test", "Job label")
		testName   = flag.String("test", "", "Test name label")
		testFile   = flag.String("file", "", "Test file label")
		batchSize  = flag.Int("batch-size", 100, "Number of lines to batch before sending")
		batchDelay = flag.Duration("batch-delay", 2*time.Second, "Maximum delay before sending a batch")
	)
	flag.Parse()

	// Parse additional labels from remaining args (format: key=value)
	labels := make(map[string]string)
	labels["job"] = *job
	if *testName != "" {
		labels["test"] = *testName
	}
	if *testFile != "" {
		labels["file"] = *testFile
	}

	// Parse any additional label=value pairs from remaining args
	for _, arg := range flag.Args() {
		if parts := strings.SplitN(arg, "=", 2); len(parts) == 2 {
			labels[parts[0]] = parts[1]
		}
	}

	scanner := bufio.NewScanner(os.Stdin)
	var batch [][]string
	lastSend := time.Now()

	// Function to send batch
	sendBatch := func() {
		if len(batch) == 0 {
			return
		}

		stream := LokiStream{
			Stream: labels,
			Values: batch,
		}

		req := LokiPushRequest{
			Streams: []LokiStream{stream},
		}

		jsonData, err := json.Marshal(req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshaling request: %v\n", err)
			return
		}

		resp, err := http.Post(*lokiURL, "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error sending to Loki: %v\n", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			fmt.Fprintf(os.Stderr, "Loki returned error: %d - %s\n", resp.StatusCode, string(body))
			return
		}

		batch = nil
		lastSend = time.Now()
	}

	// Read from stdin and batch
	for scanner.Scan() {
		line := scanner.Text()
		// Echo to stdout so user can still see output
		fmt.Println(line)

		// Add to batch with current timestamp in nanoseconds
		timestamp := fmt.Sprintf("%d", time.Now().UnixNano())
		batch = append(batch, []string{timestamp, line})

		// Send if batch is full
		if len(batch) >= *batchSize {
			sendBatch()
			continue
		}

		// Send if enough time has passed since last send and we have data
		if len(batch) > 0 && time.Since(lastSend) >= *batchDelay {
			sendBatch()
		}
	}

	// Send any remaining logs
	if len(batch) > 0 {
		sendBatch()
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
		os.Exit(1)
	}
}
