package emitter

import "fmt"

type Message struct {
	messages chan string
}

func (e *Message) Publish(msg string) {
	e.messages <- msg
}

func (e *Message) GetMessage() (string, error) {
	msg, ok := <-e.messages
	if !ok {
		return "", fmt.Errorf("channel closed")
	}
	return msg, nil
}
