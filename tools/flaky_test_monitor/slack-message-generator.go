package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/onflow/flow-go/tools/flaky_test_monitor/common"
)

func writeSummaryText(b *strings.Builder, path string, name string) {
	jsonBytes, err := os.ReadFile(path)
	common.AssertNoError(err, "error reading level 3 json")

	var summary common.TestSummary3
	err = json.Unmarshal(jsonBytes, &summary)
	common.AssertNoError(err, "error unmarshalling level 3 test run")

	fmt.Fprintf(b, "*%s*\n\n", name)

	b.WriteString("Most failures (rate):\n")

	for _, trs := range summary.MostFailures {
		fmt.Fprintf(b, "- %s: %f\n", trs.Test, trs.FailureRate)
	}

	b.WriteString("\n")
	b.WriteString("Most exceptions:\n")

	for _, trs := range summary.Exceptions {
		fmt.Fprintf(b, "- %s: %d\n", trs.Test, trs.NoResult)
	}

	b.WriteString("\n")
	b.WriteString("Longest running:\n")

	for _, trs := range summary.LongestRunning {
		fmt.Fprintf(b, "- %s: %f\n", trs.Test, trs.AverageDuration)
	}
}

func main() {
	// need to pass in single argument of where level 3 summary files exist
	if len(os.Args[1:]) != 2 {
		panic("expected path to level 3 summary files")
	}

	msg := SlackMessage{}

	unitBuilder := strings.Builder{}
	writeSummaryText(&unitBuilder, os.Args[1], "Unit Tests")
	msg.Blocks = append(msg.Blocks, Block{
		Type: "section",
		Text: Text{
			Type: "mrkdwn",
			Text: unitBuilder.String(),
		},
	})

	integrationBuilder := strings.Builder{}
	writeSummaryText(&integrationBuilder, os.Args[2], "Integration Tests")
	msg.Blocks = append(msg.Blocks, Block{
		Type: "section",
		Text: Text{
			Type: "mrkdwn",
			Text: integrationBuilder.String(),
		},
	})

	msg.Blocks = append(msg.Blocks, Block{
		Type: "section",
		Text: Text{
			Type: "mrkdwn",
			Text: fmt.Sprintf(
				"Detailed results: https://console.cloud.google.com/storage/browser/%s/SUMMARIES/%s\n",
				os.Getenv("GCS_BUCKET"),
				os.Getenv("FOLDER_NAME"),
			),
		},
	})

	common.SaveToFile("slack-message.json", msg)
}

type SlackMessage struct {
	Blocks []Block `json:"blocks"`
}

type Block struct {
	Type string `json:"type"`
	Text Text   `json:"text"`
}

type Text struct {
	Type string `json:"type"`
	Text string `json:"text"`
}
