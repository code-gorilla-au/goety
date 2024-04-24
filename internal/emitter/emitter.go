package emitter

import "fmt"

type Message struct {
	messages chan string
}

func New() *Message {
	return &Message{
		messages: make(chan string, 1),
	}
}

// Publish a message
func (e *Message) Publish(msg string) {
	e.messages <- msg
}

// GetMessage returns a message from the channel
func (e *Message) GetMessage() (string, error) {
	msg, ok := <-e.messages
	if !ok {
		return "", fmt.Errorf("channel closed")
	}
	return msg, nil
}

// Close the channel
func (e Message) Close() {
	close(e.messages)
}
