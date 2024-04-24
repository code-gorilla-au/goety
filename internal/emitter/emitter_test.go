package emitter

import (
	"testing"

	"github.com/code-gorilla-au/odize"
)

func TestMessage_Publish(t *testing.T) {
	e := New()
	defer e.Close()

	e.Publish("test")
	result := <-e.messages

	odize.AssertEqual(t, "test", result)
}

func TestMessage_GetMessage(t *testing.T) {
	e := New()
	defer e.Close()

	e.Publish("test")
	result, err := e.GetMessage()
	odize.AssertNoError(t, err)

	odize.AssertEqual(t, "test", result)
}
