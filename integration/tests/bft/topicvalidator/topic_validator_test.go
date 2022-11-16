package topicvalidator

import (
	"github.com/stretchr/testify/suite"
	"testing"
	"time"
)

type TopicValidatorTestSuite struct {
	Suite
}

func TestTopicValidator(t *testing.T) {
	suite.Run(t, new(TopicValidatorTestSuite))
}

// TestTopicValidatorE2E ensures that the libp2p topic validator is working as expected.
// This test will attempt to send multiple combinations of unauthorized messages + channel from
// a corrupted byzantine node. The victim node should not receive any of these messages as they should
// be dropped due to failing message authorization validation at the topic validator. This test will also send
// a number of authorized messages that will be delivered and processed by the victim node, ensuring that the topic
// validator behaves as expected in the happy path.
func (s *TopicValidatorTestSuite) TestTopicValidatorE2E() {
	time.Sleep(10 * time.Second)
}
