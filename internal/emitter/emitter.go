package emitter

import (
	"fmt"
	"time"
)

// Message event emitter struct
type Message struct {
	messages chan string
}

// New creates a new message emitter
func New() *Message {
	return &Message{
		messages: make(chan string, 10),
	}
}

// Publish a message (non-blocking)
func (e *Message) Publish(msg string) {
	select {
	case e.messages <- msg:
	default:
		// Channel is full, drop the message
	}
}

// PublishBlocking publishes a message and blocks until it's sent or timeout
// Use this for critical messages that must be delivered
func (e *Message) PublishBlocking(msg string) {
	select {
	case e.messages <- msg:
	case <-time.After(500 * time.Millisecond):
		// No consumer available, drop the message to prevent deadlock
	}
}

// GetMessage returns a message from the channel
func (e *Message) GetMessage() (string, error) {
	msg, ok := <-e.messages
	if !ok {
		return "", fmt.Errorf("channel closed")
	}
	return msg, nil
}

// TryGetMessage tries to get a message without blocking
// Returns empty string and false if no message available
func (e *Message) TryGetMessage() (string, bool) {
	select {
	case msg := <-e.messages:
		return msg, true
	default:
		return "", false
	}
}

// Close the channel
func (e Message) Close() {
	close(e.messages)
}
