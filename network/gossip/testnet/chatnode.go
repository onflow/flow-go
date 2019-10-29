package testnet

import (
	"context"
	"fmt"
	"sync"

	"github.com/golang/protobuf/proto"
)

type chatNode struct {
	messages []string
	mu       sync.RWMutex
}

// chatnode implements a node that sends out and receives messages as defined in message.proto

func (cn *chatNode) DisplayMessage(ctx context.Context, msg *Message) (*Void, error) {
	// disabled to not clutter up space in console
	// fmt.Printf("\n%v: %v", msg.Sender, string(msg.Content))
	cn.mu.Lock()
	cn.messages = append(cn.messages, string(msg.Content))
	cn.mu.Unlock()
	return &Void{}, nil
}

// Initializes a chatNode
func newChatNode() (*chatNode, error) {
	return &chatNode{messages: make([]string, 0)}, nil
}

// createMsg constructs a Message and marshals it
func createMsg(content, sender string) ([]byte, error) {
	msg := &Message{
		Sender:  sender,
		Content: []byte(content),
	}

	msgBytes, err := proto.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("could not marshal proto message: %v", err)
	}
	return msgBytes, nil
}
